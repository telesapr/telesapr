// agent-client.js: a program's end of TeleSAPR's AI connection in a
// browser (doc/guide/ai.md, "In a browser"). Script > Allow AI
// Connections must be on in the page; a program of another site than
// the page's is let in only if the user allows it there.
//
//   const tele = await telesaprAgent(frame.contentWindow, "https://example.org");
//   const out = await tele.run("fmt.Println(len(doc.Groups()))"); // {output, value, error, ...}
//   const text = await tele.describe("");                      // the drawing in words
//   tele.close();
//
// call(op, fields) sends any request of the protocol (pkg/script/agent,
// wire.go): run, describe, render, help, measure, export, open, files.
async function telesaprAgent(target = window, origin = location.origin) {
  const channel = new MessageChannel();
  const port = channel.port1;
  const waiting = new Map();
  let next = 1;
  const ready = new Promise((resolve, reject) => {
    port.onmessage = (ev) => {
      const d = ev.data;
      if (typeof d === "string") {
        const r = JSON.parse(d);
        const w = waiting.get(r.id);
        if (w) {
          waiting.delete(r.id);
          if (r.error) w.reject(new Error(r.error));
          else w.resolve(r);
        }
      } else if (d && d.type === "telesapr-agent-ready") {
        resolve(d.protocol);
      } else if (d && d.type === "telesapr-agent-refused") {
        reject(new Error(d.error));
      }
    };
  });
  target.postMessage({ type: "telesapr-agent-connect" }, origin, [channel.port2]);
  const protocol = await ready;
  const call = (op, fields = {}) =>
    new Promise((resolve, reject) => {
      const id = next++;
      waiting.set(id, { resolve, reject });
      port.postMessage(JSON.stringify({ id, op, ...fields }));
    });
  return {
    protocol,
    call,
    run: (code) => call("run", { code }).then((r) => r.result),
    describe: (group = "") => call("describe", { group }).then((r) => r.text),
    help: (name = "") => call("help", { name }).then((r) => r.text),
    measure: (group = "") => call("measure", { group }).then((r) => r.text),
    close() {
      port.postMessage({ type: "telesapr-agent-close" });
      port.close();
    },
  };
}
