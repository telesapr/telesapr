// agent-client.js: a program's end of TeleSAPR's AI connection in a
// browser (doc/guide/ai.md, "In a browser"). Script > Allow AI
// Connections must be on in the page; a program of another site than
// the page's is let in only if the user allows it there.
//
//   const tele = await telesaprAgent(frame.contentWindow, "https://example.org");
//   const out = await tele.run("fmt.Println(len(doc.Groups()))"); // {output, value, error, ...}
//   await tele.run(code, (p) => console.log(p.message, p.done, p.total, p.output)); // while it runs
//   const text = await tele.describe("");                      // the drawing in words
//   const { data } = await tele.described("");                 // and as data: {unit, groups, ...}
//   tele.close();
//
// call(op, fields) sends any request of the protocol (pkg/script/agent,
// wire.go): run, describe, render, help, measure, export, open, files;
// onProgress, if given, is called with each progress line of a run.
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
        if (w && r.op === "progress") {
          if (w.onProgress) w.onProgress(r.progress || {});
        } else if (w) {
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
  const call = (op, fields = {}, onProgress = null) =>
    new Promise((resolve, reject) => {
      const id = next++;
      waiting.set(id, { resolve, reject, onProgress });
      port.postMessage(JSON.stringify({ id, op, ...fields }));
    });
  return {
    protocol,
    call,
    run: (code, onProgress = null) => call("run", { code }, onProgress).then((r) => r.result),
    describe: (group = "", script = false) => call("describe", { group, script }).then((r) => r.text),
    help: (name = "") => call("help", { name }).then((r) => r.text),
    measure: (group = "") => call("measure", { group }).then((r) => r.text),
    // described and measured are describe and measure with the same as
    // data beside the text: {text, data}.
    described: (group = "", script = false, solid = false) =>
      call("describe", { group, script, solid }).then((r) => ({ text: r.text, data: r.data })),
    measured: (group = "") => call("measure", { group }).then((r) => ({ text: r.text, data: r.data })),
    close() {
      port.postMessage({ type: "telesapr-agent-close" });
      port.close();
    },
  };
}
