const net = require("net");
const server = net.createServer();

const port = process.env.PORT || 9000;

server.on("connection", handleConnection);

server.listen(port, function () {
  console.log(`server listening to ${server.address()}`);
});

function handleConnection(conn) {
  const remoteAddress = `${conn.remoteAddress}:${conn.remotePort}`;
  console.log(`new client connection from ${remoteAddress}`);

  conn.setEncoding("utf8");

  conn.on("data", onConnData);
  conn.once("close", onConnClose);
  conn.on("error", onConnError);

  function onConnData(d) {
    console.log(`connection data from ${remoteAddress}: ${d}`);
    conn.write(`Hello ${d}`);
  }

  function onConnClose() {
    console.log(`connection from ${remoteAddress} closed`);
  }

  function onConnError(err) {
    console.log(`Connection ${remoteAddress} error: ${err.message}`);
  }
}
