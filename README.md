# Wind

Wind 是一个 Golang 微服务 HTTP 框架，在设计之初参考了其他开源框架 [hertz](https://github.com/cloudwego/hertz)、 [fasthttp](https://github.com/valyala/fasthttp)、[gin](https://github.com/gin-gonic/gin)、[echo](https://github.com/labstack/echo)  的优势，并结合内部的需求，使其具有高易用性、高性能、高扩展性等特点，目前在内部已广泛使用。如今越来越多的微服务选择使用 Golang，如果对微服务性能有要求，又希望框架能够充分满足内部的可定制性需求，Wind 会是一个不过的选择。

## 框架特点
- 高易用性

  在开发过程中，快速写出正确的代码往往是更重要的。因此，在 Wind 的迭代过程中，积极挺起用户意见，持续打磨框架，希望为用户提供一个更好的使用体验，帮助用户更快的写出正确的代码。

- 高性能
  
  Wind 默认使用字节跳动的高性能网络库 Netpoll，在一些特殊场景相较于 go net，Wind 在 QPS、时延上均具有一定优势。关于性能数据，可参考下图 Echo 数据。

  四个框架的对比： wind/fasthttp/fiber/gin
  三个框架的对比： wind/fasthttp/fiber

- 高扩展性
  Wind 采用了分层设计，提供了较多的接口及默认实现，用户亦可自行扩展。同时得益于分层设计，框架的扩展性也会大很多。更多的扩展规划参考 [RoadMap](ROADMAP.md)

- 多协议支持
  Wind 框架原生提供了 HTTP/1.1、HTTP/2、HTTP/3 及 ALPN 协议支持。除此之外，由于分层设计，Wind 甚至支持自定义构建协议解析逻辑，以满足协议层扩展的任意需求。

- 网络切换能力
  Wind 实现了 Netpoll 和 Golang 原生网络库之间的按需切换能力。用户可针对不同场景选择合适的网络库。同时也支持以插件的方式为 Wind 扩展网络实现。


## 相关扩展

| 扩展        | 描述                         |
|-----------|----------------------------|
| Autotls   | 为 Wind 提供 Let's Encrypt 支持 |
| Http2     | 为 Wind 提供 HTTP2 支持         |
| Websocket | 为 Wind 提供 Websocket 支持     |
