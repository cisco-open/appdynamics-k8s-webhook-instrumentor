// const http = require("http");
const express = require('express')
const app = express()

const host = '0.0.0.0';
const port = 8000;

/*
const requestListener = function (req, res) {
  res.writeHead(200);
  res.end("My test server!");
};

const server = http.createServer(requestListener);
server.listen(port, host, () => {
    console.log(`Server is running on http://${host}:${port}`);
});
*/

app.get('/', (req, res) => {
  res.send('Hello World!')
})

app.get('/api/search', async (req, res) => {
  await new Promise(r => setTimeout(r, randomDelay(500, 20)));
  res.send('Hello API!')
})

app.get('/api/tool', async (req, res) => {
  await new Promise(r => setTimeout(r, randomDelay(1000, 20)));
  res.send('Hello Tool!')
})

app.get('/api/delete/:id/now', async (req, res) => {
  await new Promise(r => setTimeout(r, randomDelay(1000, 20)));
  res.send('Hello delete!')
})

app.listen(port, () => {
  console.log(`Example app listening on port ${port}`)
})

function randomDelay(ms, randPct) {
  const fluct = ms * randPct / 100 
  return ms + Math.floor(Math.random() * fluct) - fluct/2;
}