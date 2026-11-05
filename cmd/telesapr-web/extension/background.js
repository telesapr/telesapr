// background.js: the extension's middle. While a TeleSAPR page is
// open it keeps telesapr-cli browser-relay running by native messaging
// (pkg/script/agent, relay.go), and gives each connection the relay
// opens to the page that offered itself last. A line the relay sends
// in parts ("more") is put together here.
"use strict";

const host = "org.telesapr.relay";

let relay = null;
const pages = []; // the pages' ports, the last offered last
const owner = new Map(); // conn -> page port
const parts = new Map(); // conn -> the line so far

chrome.runtime.onConnect.addListener((page) => {
  if (page.name !== "telesapr-page") return;
  pages.push(page);
  page.onMessage.addListener((m) => {
    // A page speaks only for its own connections.
    if (owner.get(m.conn) !== page || !relay) return;
    if (m.op === "close") forget(m.conn);
    relay.postMessage(m);
  });
  page.onDisconnect.addListener(() => {
    pages.splice(pages.indexOf(page), 1);
    for (const [conn, p] of [...owner]) {
      if (p !== page) continue;
      forget(conn);
      if (relay) relay.postMessage({ conn, op: "close", error: "the TeleSAPR page was closed" });
    }
    if (pages.length === 0 && relay) {
      relay.disconnect();
      relay = null;
    }
  });
  if (!relay) start();
});

function forget(conn) {
  owner.delete(conn);
  parts.delete(conn);
}

function start() {
  relay = chrome.runtime.connectNative(host);
  const self = relay;
  relay.onMessage.addListener((m) => {
    if (m.op === "open") {
      const page = pages[pages.length - 1];
      if (!page) {
        relay.postMessage({ conn: m.conn, op: "close", error: "no TeleSAPR page is open" });
        return;
      }
      owner.set(m.conn, page);
      page.postMessage(m);
      return;
    }
    const page = owner.get(m.conn);
    if (!page) return;
    if (m.op === "line") {
      const text = (parts.get(m.conn) || "") + m.text;
      if (m.more) {
        parts.set(m.conn, text);
        return;
      }
      parts.delete(m.conn);
      page.postMessage({ conn: m.conn, op: "line", text });
    } else if (m.op === "close") {
      forget(m.conn);
      page.postMessage(m);
    }
  });
  relay.onDisconnect.addListener(() => {
    const why = chrome.runtime.lastError;
    if (why) console.warn(`TeleSAPR relay (${host}): ${why.message}; run telesapr-cli browser-relay --install`);
    if (relay !== self) return;
    relay = null;
    for (const [conn, page] of [...owner]) {
      forget(conn);
      page.postMessage({ conn, op: "close" });
    }
  });
}
