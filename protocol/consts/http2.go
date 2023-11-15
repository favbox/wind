package consts

// ClientPreface HTTP/2 协议中的一个特殊的帧。
// 客户端发送此帧表明其希望升级到 HTTP/2.0 协议。
const ClientPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
