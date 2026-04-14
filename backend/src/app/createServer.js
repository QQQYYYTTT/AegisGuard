const http = require("http");
const { serveFrontend, sendJson } = require("../lib/http");
const { handleApiRequest } = require("../routes/apiRouter");

function createServer() {
  return http.createServer(async (req, res) => {
    const url = new URL(req.url, `http://${req.headers.host}`);
    const pathname = decodeURIComponent(url.pathname);

    try {
      const handled = await handleApiRequest(req, res, pathname);
      if (!handled) {
        serveFrontend(res, pathname);
      }
    } catch (error) {
      sendJson(res, 500, { error: error.message || "Server error" });
    }
  });
}

module.exports = {
  createServer
};
