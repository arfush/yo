// Yonime watch-party relay.
//
// One Durable Object instance per room key keeps the last host anchor in
// memory and forwards it to the guests. The class deliberately does not use
// hibernatable WebSockets: a room lives for the length of an episode and the
// in-memory state is what makes a late joiner start at the right second.
//
// Wire format is shared with the client in internal/watchparty:
//
//	{t:"hello"}                 guest asks for a fresh anchor
//	{t:"state", state:{...}}    host publishes an anchor
//	{t:"welcome", host:true}    relay reports whether the host is present
//	{t:"bye"}                   host left
//	{t:"error", error:"..."}    relay refused something

const KEY_LENGTH = 9;
const KEY_PATTERN = /^YO[A-Z]{7}$/;

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (url.pathname === "/health") {
      return new Response("ok", { headers: { "content-type": "text/plain" } });
    }
    if (url.pathname !== "/room") {
      return new Response(
        "Yonime watch-party relay. Connect to /room?key=<KEY>&role=host|guest",
        { status: 200, headers: { "content-type": "text/plain; charset=utf-8" } },
      );
    }
    if (request.headers.get("Upgrade") !== "websocket") {
      return new Response("expected a WebSocket upgrade", { status: 426 });
    }

    const key = normalizeKey(url.searchParams.get("key") ?? "");
    if (!key) {
      return new Response("bad room key", { status: 400 });
    }
    const role = url.searchParams.get("role") === "host" ? "host" : "guest";

    const id = env.ROOMS.idFromName(key);
    const stub = env.ROOMS.get(id);
    const target = new URL("https://room.internal/ws");
    target.searchParams.set("key", key);
    target.searchParams.set("role", role);
    return stub.fetch(target.toString(), request);
  },
};

function normalizeKey(value) {
  const key = value.trim().toUpperCase();
  if (key.length !== KEY_LENGTH || !KEY_PATTERN.test(key)) {
    return "";
  }
  return key;
}

export class Room {
  constructor(state) {
    this.state = state;
    this.host = null;
    this.guests = new Set();
    // lastState is the raw host frame, forwarded verbatim to a new guest.
    this.lastState = null;
  }

  async fetch(request) {
    const url = new URL(request.url);
    const role = url.searchParams.get("role") === "host" ? "host" : "guest";

    const pair = new WebSocketPair();
    const client = pair[0];
    const server = pair[1];
    server.accept();

    if (role === "host") {
      this.attachHost(server);
    } else {
      this.attachGuest(server);
    }

    return new Response(null, { status: 101, webSocket: client });
  }

  attachHost(socket) {
    // A second host takes the room over: the old tab was probably reloaded
    // without a clean close.
    if (this.host) {
      try {
        this.host.close(4001, "replaced by another host");
      } catch {
        // The socket may already be gone.
      }
    }
    this.host = socket;
    // A new host session starts from an empty room; the client publishes its
    // own anchor right after connecting.
    this.lastState = null;
    this.send(socket, { t: "welcome", host: true });

    socket.addEventListener("message", (event) => {
      if (this.host !== socket) {
        return;
      }
      const message = parse(event.data);
      if (!message) {
        return;
      }
      switch (message.t) {
        case "state":
          this.lastState = event.data;
          this.broadcast(message, socket);
          break;
        case "bye":
          this.dropHost(socket);
          break;
        default:
          break;
      }
    });

    const onGone = () => {
      if (this.host === socket) {
        this.dropHost(socket);
      }
    };
    socket.addEventListener("close", onGone);
    socket.addEventListener("error", onGone);
  }

  attachGuest(socket) {
    this.guests.add(socket);
    const hosted = this.host !== null;
    this.send(socket, { t: "welcome", host: hosted });
    if (hosted && this.lastState) {
      socket.send(this.lastState);
    }

    socket.addEventListener("message", (event) => {
      const message = parse(event.data);
      if (!message || message.t !== "hello") {
        return;
      }
      if (this.host) {
        // Ask for a fresh anchor instead of replaying a stale one, so a guest
        // joining mid-episode lands on the current second.
        this.send(this.host, { t: "hello", role: "guest" });
      } else {
        this.send(socket, { t: "welcome", host: false });
      }
    });

    const onGone = () => {
      this.guests.delete(socket);
    };
    socket.addEventListener("close", onGone);
    socket.addEventListener("error", onGone);
  }

  dropHost(socket) {
    if (this.host !== socket) {
      // A replaced or already dropped host must not clear the current one.
      return;
    }
    this.host = null;
    this.lastState = null;
    try {
      socket.close(1000, "host left");
    } catch {
      // Already closed.
    }
    for (const guest of this.guests) {
      this.send(guest, { t: "bye" });
    }
  }

  broadcast(message, except) {
    for (const guest of this.guests) {
      if (guest !== except) {
        this.send(guest, message);
      }
    }
  }

  send(socket, message) {
    try {
      socket.send(JSON.stringify(message));
    } catch {
      if (socket === this.host) {
        this.host = null;
      } else {
        this.guests.delete(socket);
      }
    }
  }
}

function parse(data) {
  if (typeof data !== "string" || data.length === 0 || data.length > 64 * 1024) {
    return null;
  }
  try {
    return JSON.parse(data);
  } catch {
    return null;
  }
}
