-- Yonime header overlay for mpv.
--
-- Draws the application's window header (logo, title, version badge and
-- minimise/maximise/close buttons) over the top of the mpv window. The native
-- title bar is disabled by the launcher; the empty header area keeps mpv's
-- built-in window dragging, while the three buttons get their own input
-- sections so a click does not start a drag.

local mp = require 'mp'
local opt = require 'mp.options'
local assdraw = require 'mp.assdraw'

local options = {
    title = "Yonime",
    version = "",
    -- Header height in device-independent pixels, matching ui.headerHeight.
    height = 52,
    -- Font family loaded from the fonts directory passed by the launcher.
    font = "Inter",
}

opt.read_options(options, "yonime-header")

local HEADER_HEIGHT = tonumber(options.height) or 52
local HEADER_FONT = options.font or "Inter"
local TITLE = options.title or "Yonime"
local VERSION = options.version or ""

-- Palette copied from ui/theme.go.
local COLORS = {
    header = "#111219",
    divider = "#1b1c23",
    title = "#f4f2fa",
    icon = "#9683e7",
    badgeFill = "#29223e",
    badgeBorder = "#39304f",
    badgeText = "#9683e7",
    hover = "#9a9a9f",
    closeHover = "#ff6473",
    logoBackground = "#14121d",
    logoGate = "#ad9eff",
}

-- Advance widths of the version characters at 11 sp in Inter Medium. They
-- match the ink widths Gio measures for the application's badge.
local BADGE_ADVANCE = {
    ["0"] = 8, ["1"] = 5, ["2"] = 7, ["3"] = 7, ["4"] = 8,
    ["5"] = 7, ["6"] = 7, ["7"] = 7, ["8"] = 8, ["9"] = 8,
    ["v"] = 6, ["V"] = 7, ["."] = 3, ["-"] = 4, ["+"] = 7,
}

local overlay = mp.create_osd_overlay("ass-events")
overlay.z = 1100

local state = {
    width = 0,
    height = 0,
    scale = 1,
    header = 0,
    border = 1,
    divider = 1,
    titleY = 0,
    iconY = 0,
    textScale = 1,
    hovered = nil,
    hover = {},
    visible = false,
    fullscreen = false,
    maximized = false,
    rendered = "",
}

local CONTROL_NAMES = { "minimize", "maximize", "close" }
local SECTIONS = {
    minimize = "yonime-header-minimize",
    maximize = "yonime-header-maximize",
    close = "yonime-header-close",
}

-- ---------------------------------------------------------------------------
-- ASS helpers

local function round(v)
    return math.floor(v + 0.5)
end

local function px(v)
    return round(v * state.scale)
end

local function least_px(v)
    local n = px(v)
    if n < 1 then
        return 1
    end
    return n
end

local function parse_hex(hex)
    return tonumber(hex:sub(2, 3), 16) or 0,
        tonumber(hex:sub(4, 5), 16) or 0,
        tonumber(hex:sub(6, 7), 16) or 0
end

-- ASS colours are BGR, with an optional leading alpha. opacity=1 is opaque.
local function color_tag(hex, opacity)
    local r, g, b = parse_hex(hex)
    local alpha = 0
    if opacity and opacity < 1 then
        alpha = 255 - round(math.max(0, opacity) * 255)
    end
    return string.format("\\1c&H%02X%02X%02X%02X&", alpha, b, g, r)
end

-- Starts a filled vector shape positioned at the window origin.
local function shape(ass, color, opacity)
    ass:new_event()
    ass:append("{\\an7\\pos(0,0)\\bord0\\shad0" .. color_tag(color, opacity) .. "}")
    ass:draw_start()
end

local function append_rect(ass, x0, y0, x1, y1)
    ass:move_to(x0, y0)
    ass:line_to(x1, y0)
    ass:line_to(x1, y1)
    ass:line_to(x0, y1)
end

local function draw_rounded(ass, x0, y0, x1, y1, radius, color, opacity)
    if x1 - x0 <= 0 or y1 - y0 <= 0 then
        return
    end
    shape(ass, color, opacity)
    ass:round_rect_cw(x0, y0, x1, y1, math.max(0, radius))
    ass:draw_stop()
end

-- A line as a filled quad so its thickness is exact at any DPI.
local function draw_line(ass, x1, y1, x2, y2, thickness, color)
    local dx, dy = x2 - x1, y2 - y1
    local length = math.sqrt(dx * dx + dy * dy)
    if length <= 0 then
        return
    end
    local nx = -dy / length * thickness / 2
    local ny = dx / length * thickness / 2
    shape(ass, color or COLORS.icon)
    ass:move_to(x1 + nx, y1 + ny)
    ass:line_to(x2 + nx, y2 + ny)
    ass:line_to(x2 - nx, y2 - ny)
    ass:line_to(x1 - nx, y1 - ny)
    ass:draw_stop()
end

local draw_rect
local function draw_outline_rect(ass, x0, y0, x1, y1, thickness)
    draw_rect(ass, x0, y0, x1, y0 + thickness)
    draw_rect(ass, x0, y1 - thickness, x1, y1)
    draw_rect(ass, x0, y0, x0 + thickness, y1)
    draw_rect(ass, x1 - thickness, y0, x1, y1)
end

draw_rect = function(ass, x0, y0, x1, y1, color, opacity)
    if x1 - x0 <= 0 or y1 - y0 <= 0 then
        return
    end
    shape(ass, color or COLORS.icon, opacity)
    append_rect(ass, x0, y0, x1, y1)
    ass:draw_stop()
end

-- ---------------------------------------------------------------------------
-- Geometry

local function button_rects()
    local s = state.scale
    local width = state.width
    local header = state.header
    local border = state.border
    local control = px(30)
    local gap = px(12)
    local padding = px(12)
    local right = width - border - padding
    local close = {
        x0 = right - control, x1 = right, y0 = 0, y1 = header,
        cx = right - control + math.floor(control / 2), cy = state.iconY,
    }
    local maximize = {
        x0 = close.x0 - gap - control, x1 = close.x0 - gap, y0 = 0, y1 = header,
        cx = close.x0 - gap - control + math.floor(control / 2), cy = state.iconY,
    }
    local minimize = {
        x0 = maximize.x0 - gap - control, x1 = maximize.x0 - gap, y0 = 0, y1 = header,
        cx = maximize.x0 - gap - control + math.floor(control / 2), cy = state.iconY,
    }
    return { minimize = minimize, maximize = maximize, close = close }
end

local function control_hover_rect(rect)
    local size = px(14)
    return rect.cx - size, rect.cy - size, rect.cx + size, rect.cy + size
end

-- ---------------------------------------------------------------------------
-- Drawing

local function draw_logo(ass, x, y, size)
    local radius = size * 0.21
    draw_rounded(ass, x, y, x + size, y + size, radius, COLORS.logoBackground)

    -- The gate is traced from ui/assets/app-icon-48.png: a top lintel, a
    -- centre strut, a second beam, and two posts, all in icon lavender.
    local u = size / 48
    local gate = COLORS.logoGate

    -- Top beam.
    draw_rect(ass, x + 9 * u, y + 13 * u, x + 37 * u, y + 17 * u, gate)
    -- Centre strut and the upper parts of both posts.
    draw_rect(ass, x + 13 * u, y + 17 * u, x + 18 * u, y + 23 * u, gate)
    draw_rect(ass, x + 21 * u, y + 17 * u, x + 25 * u, y + 23 * u, gate)
    draw_rect(ass, x + 29 * u, y + 17 * u, x + 34 * u, y + 23 * u, gate)
    -- Second beam.
    draw_rect(ass, x + 10 * u, y + 23 * u, x + 34 * u, y + 27 * u, gate)
    -- Posts.
    draw_rect(ass, x + 13 * u, y + 27 * u, x + 18 * u, y + 35 * u, gate)
    draw_rect(ass, x + 29 * u, y + 27 * u, x + 34 * u, y + 35 * u, gate)
end

local function draw_icon(ass, name, rect)
    local cx, cy = rect.cx, rect.cy
    local s = state.scale
    local thickness = least_px(1)

    if name == "minimize" then
        local top = cy - math.floor(thickness / 2)
        draw_rect(ass, cx - px(5), top, cx + px(5), top + thickness)
    elseif name == "maximize" then
        if state.maximized then
            local back = { cx - px(3), cy - px(6), cx + px(6), cy + px(3) }
            local front = { cx - px(6), cy - px(3), cx + px(3), cy + px(6) }
            -- The rear square is partially hidden by the front one: draw only
            -- the edges that remain visible, as ui/icons.go does.
            draw_rect(ass, back[1], back[2], back[3], back[2] + thickness)
            draw_rect(ass, back[3] - thickness, back[2], back[3], back[4])
            draw_rect(ass, front[3], back[4] - thickness, back[3], back[4])
            draw_rect(ass, back[1], back[2], back[1] + thickness, front[2])
            draw_outline_rect(ass, front[1], front[2], front[3], front[4], thickness)
        else
            draw_outline_rect(ass, cx - px(5), cy - px(5), cx + px(5), cy + px(5), thickness)
        end
    elseif name == "close" then
        draw_line(ass, cx - 4.5 * s, cy - 4.5 * s, cx + 4.5 * s, cy + 4.5 * s, 1.1 * s)
        draw_line(ass, cx + 4.5 * s, cy - 4.5 * s, cx - 4.5 * s, cy + 4.5 * s, 1.1 * s)
    end
end

local function button(ass, name, rect)
    local hover = state.hover[name]
    local progress = hover and hover.progress or 0
    if progress > 0.002 then
        local color = name == "close" and COLORS.closeHover or COLORS.hover
        local x0, y0, x1, y1 = control_hover_rect(rect)
        local radius = px(7)
        draw_rounded(ass, x0, y0, x1, y1, radius, color, progress)
        local inset = least_px(1)
        draw_rounded(
            ass,
            x0 + inset, y0 + inset, x1 - inset, y1 - inset,
            math.max(0, radius - inset),
            COLORS.header
        )
    end
    draw_icon(ass, name, rect)
end

local function badge_text_width()
    local width = 0
    for i = 1, #VERSION do
        local char = VERSION:sub(i, i)
        width = width + (BADGE_ADVANCE[char] or 7)
    end
    return math.max(20, width)
end

local function render()
    if not state.visible or state.width <= 0 or state.height <= 0 then
        return
    end
    local w, h = state.width, state.height
    local s = state.scale
    local header = state.header
    local ass = assdraw.ass_new()

    -- Header background and its divider. The video starts below the divider,
    -- just as the application's content rectangle does.
    draw_rect(ass, 0, 0, w, header, COLORS.header)
    draw_rect(ass, 0, header, w, header + state.divider, COLORS.divider)

    -- Logo and title.
    local logo_size = px(24)
    draw_logo(ass, px(19), state.titleY - logo_size / 2, logo_size)

    local text_scale = state.textScale
    ass:new_event()
    ass:append(string.format(
        "{\\an4\\pos(%d,%d)\\fn%s\\fs%f\\b1\\bord0\\shad0%s}%s",
        px(49), state.titleY, HEADER_FONT, 13.2 * s * text_scale,
        color_tag(COLORS.title), TITLE
    ))

    -- Version badge. ASS cannot measure text, so the width comes from the
    -- same character advances the badge text needs at 11 sp.
    if VERSION ~= "" then
        local badge_height = px(20)
        local border = least_px(1)
        local pad = px(7)
        local text_width = badge_text_width() * s
        local badge_x = px(110)
        local badge_y = state.titleY - badge_height / 2
        local badge_width = text_width + 2 * pad + 2 * border

        draw_rounded(
            ass, badge_x, badge_y, badge_x + badge_width, badge_y + badge_height,
            badge_height / 2, COLORS.badgeBorder
        )
        draw_rounded(
            ass,
            badge_x + border, badge_y + border,
            badge_x + badge_width - border, badge_y + badge_height - border,
            math.max(0, badge_height / 2 - border),
            COLORS.badgeFill
        )
        -- Centre the text in the pill. \an5 anchors the glyphs by their centre,
        -- so versions of different widths stay visually centred in mpv too.
        ass:new_event()
        ass:append(string.format(
            "{\\an5\\pos(%d,%d)\\fn%s Medium\\fs%f\\b0\\bord0\\shad0%s}%s",
            round(badge_x + badge_width / 2), state.titleY, HEADER_FONT, 11 * s * text_scale,
            color_tag(COLORS.badgeText), VERSION
        ))
    end

    -- Window controls.
    local rects = button_rects()
    for _, name in ipairs(CONTROL_NAMES) do
        button(ass, name, rects[name])
    end

    local data = ass.text
    if data ~= state.rendered then
        state.rendered = data
        overlay.res_x = math.max(1, math.floor(w))
        overlay.res_y = math.max(1, math.floor(h))
        overlay.data = data
        overlay:update()
    end
end

-- ---------------------------------------------------------------------------
-- Hover animation

local function animate_hover()
    local now = mp.get_time()
    local running = false
    for _, name in ipairs(CONTROL_NAMES) do
        local h = state.hover[name]
        if h and h.running then
            local t = (now - h.start) / 0.14
            if t >= 1 then
                t = 1
                h.running = false
            end
            local eased = t * t * (3 - 2 * t)
            h.progress = h.from + (h.target - h.from) * eased
            if h.running then
                running = true
            end
        end
    end
    render()
    if running then
        mp.add_timeout(0.016, animate_hover)
    end
end

local function set_hovered(name)
    if name == state.hovered then
        return
    end
    state.hovered = name
    local now = mp.get_time()
    for _, control in ipairs(CONTROL_NAMES) do
        local h = state.hover[control]
        if not h then
            h = { progress = 0, target = 0, from = 0, start = 0, running = false }
            state.hover[control] = h
        end
        local target = (control == name) and 1 or 0
        if target ~= h.target then
            h.from = h.progress
            h.target = target
            h.start = now
            h.running = true
        end
    end
    animate_hover()
end

-- ---------------------------------------------------------------------------
-- Layout and state

local function update_mouse_areas()
    local rects = button_rects()
    for name, section in pairs(SECTIONS) do
        local rect = rects[name]
        mp.set_mouse_area(rect.x0, rect.y0, rect.x1, rect.y1, section)
    end
end

local function update_video_margin()
    if not state.visible or state.fullscreen or state.height <= 0 then
        mp.set_property_number("video-margin-ratio-top", 0)
        return
    end
    local top = state.header + state.divider
    mp.set_property_number("video-margin-ratio-top", top / state.height)
end

-- libass renders OSD text smaller than its nominal \fs size: measured against
-- the bundled mpv build, a 100 px glyph is drawn at ~70 px. The factor does not
-- depend on the window size, the video aspect, the OSD margins or the DPI, while
-- vector drawings stay exactly 1:1. Scale text by the inverse so the title and
-- badge keep their design sizes at every window size.
local TEXT_SCALE = 1 / 0.7025

local function update_scale()
    local scale = mp.get_property_number("display-hidpi-scale", 1)
    if not scale or scale <= 0 then
        scale = 1
    end
    state.scale = scale
    state.border = least_px(1)
    state.divider = least_px(1)
    state.header = round(HEADER_HEIGHT * scale)
    state.titleY = math.floor((state.border + state.header) / 2)
    state.iconY = math.floor(state.header / 2)
end

local function refresh()
    update_scale()
    local width = mp.get_property_number("osd-width", 0) or 0
    local height = mp.get_property_number("osd-height", 0) or 0
    state.width = math.floor(width)
    state.height = math.floor(height)
    state.maximized = mp.get_property_bool("window-maximized", false)
    state.fullscreen = mp.get_property_bool("fullscreen", false)

    if state.fullscreen then
        state.visible = false
        state.rendered = ""
        overlay:remove()
        update_video_margin()
        return
    end

    if state.width <= 0 or state.height <= 0 then
        state.visible = false
        state.rendered = ""
        overlay:remove()
        update_video_margin()
        return
    end

    state.visible = true
    update_mouse_areas()
    update_video_margin()
    state.textScale = TEXT_SCALE
    render()
end

local refresh_timer = mp.add_timeout(0.02, refresh)
local function request_refresh()
    if not refresh_timer:is_enabled() then
        refresh_timer:resume()
    end
end

-- mouse-pos is in OSD/window coordinates.
local function update_hover()
    local pos = mp.get_property_native("mouse-pos")
    if not pos or not state.visible or state.fullscreen then
        set_hovered(nil)
        return
    end
    local rects = button_rects()
    local hovered = nil
    for _, name in ipairs(CONTROL_NAMES) do
        local rect = rects[name]
        if pos.x >= rect.x0 and pos.x < rect.x1 and pos.y >= rect.y0 and pos.y < rect.y1 then
            hovered = name
            break
        end
    end
    set_hovered(hovered)
end

local function action(name)
    if name == "close" then
        mp.command("quit")
    elseif name == "minimize" then
        mp.command("cycle window-minimized")
    elseif name == "maximize" then
        if not mp.get_property_bool("fullscreen", false) then
            mp.command("cycle window-maximized")
        end
    end
end

for name, section in pairs(SECTIONS) do
    mp.set_key_bindings({
        { "mbtn_left", function() action(name) end },
        { "mbtn_left_dbl", function() end },
    }, section)
    mp.enable_key_bindings(section)
end

mp.observe_property("osd-width", "number", request_refresh)
mp.observe_property("osd-height", "number", request_refresh)
mp.observe_property("display-hidpi-scale", "number", request_refresh)
mp.observe_property("fullscreen", "bool", request_refresh)
mp.observe_property("window-maximized", "bool", request_refresh)
mp.observe_property("file-loaded", "none", request_refresh)

-- mouse-pos does not emit change notifications, so poll it while the window is
-- visible. 40 ms matches ui/hover.go's cursor polling interval.
mp.add_periodic_timer(0.04, update_hover)

-- The window geometry and properties are not final during script load.
mp.add_timeout(0.1, refresh)

mp.register_script_message("yonime-header-refresh", refresh)
