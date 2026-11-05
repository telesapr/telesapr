// content.js: the extension's end in a TeleSAPR page. Each connection
// the relay opens (background.js) is carried to the page's
// postMessage as a program of the page's own site, which the page lets
// in while Script > Allow AI Connections is on (pkg/script/agent,
// socket_js.go). Messages with the background are
// {conn, op: "open" | "ready" | "line" | "close", text, error}.
"use strict";

// How long the page has to take a connection before it is taken to
// have AI connections off: the page doesn't answer then at all.
const answerWithin = 3000;

let background = null;
const conns = new Map(); // conn -> {port, ready, queue, timer}

function connect() {
  background = chrome.runtime.connect({ name: "telesapr-page" });
  background.onMessage.addListener(fromBackground);
  background.onDisconnect.addListener(() => {
    // The background was stopped or the extension reloaded: what went
    // through it ended, and this page offers itself again.
    for (const conn of [...conns.keys()]) end(conn, null);
    background = null;
    setTimeout(connect, 1000);
  });
}

function fromBackground(m) {
  switch (m.op) {
    case "open":
      open(m.conn);
      break;
    case "line": {
      const c = conns.get(m.conn);
      if (!c) return;
      if (c.ready) c.port.postMessage(m.text);
      else c.queue.push(m.text);
      break;
    }
    case "close":
      end(m.conn, null);
      break;
  }
}

function send(m) {
  if (background) background.postMessage(m);
}

function open(conn) {
  const channel = new MessageChannel();
  const c = { port: channel.port1, ready: false, queue: [], timer: 0 };
  conns.set(conn, c);
  c.timer = setTimeout(
    () => end(conn, "the TeleSAPR page didn't answer: turn on Script > Allow AI Connections in it"),
    answerWithin,
  );
  c.port.onmessage = (ev) => {
    const d = ev.data;
    if (typeof d === "string") {
      send({ conn, op: "line", text: d });
    } else if (d && d.type === "telesapr-agent-ready") {
      clearTimeout(c.timer);
      c.ready = true;
      send({ conn, op: "ready" });
      for (const line of c.queue) c.port.postMessage(line);
      c.queue = [];
    } else if (d && d.type === "telesapr-agent-refused") {
      end(conn, d.error || "the TeleSAPR page refused the connection");
    }
  };
  window.postMessage({ type: "telesapr-agent-connect" }, location.origin, [channel.port2]);
}

// end ends a connection; with an error, the relay is told why.
function end(conn, error) {
  const c = conns.get(conn);
  if (!c) return;
  conns.delete(conn);
  clearTimeout(c.timer);
  if (c.ready) c.port.postMessage({ type: "telesapr-agent-close" });
  c.port.close();
  send(error ? { conn, op: "close", error } : { conn, op: "close" });
}

// Only TeleSAPR's page offers itself: the extension also runs on every
// page served from this machine, where make wasm's is tried.
if (document.querySelector('meta[name="telesapr"][content="editor"]')) connect();
