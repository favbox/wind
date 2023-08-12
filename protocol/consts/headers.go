package consts

const (
	HeaderDate = "Date"

	HeaderIfModifiedSince = "If-Modified-Since"
	HeaderLastModified    = "Last-Modified"

	HeaderLocation = "Location" // 重定向

	HeaderVary = "Vary"
)

// 传输编码类
const (
	HeaderTE               = "TE"
	HeaderTrailer          = "Trailer"
	HeaderTrailerLower     = "trailer"
	HeaderTransferEncoding = "Transfer-Encoding"
)

// 控制类
const (
	HeaderCookie         = "Cookie"
	HeaderExpect         = "Expect"
	HeaderMaxForwards    = "Max-Forwards"
	HeaderSetCookie      = "Set-Cookie"
	HeaderSetCookieLower = "set-cookie"
)

// 连接管理类
const (
	HeaderConnection      = "Connection"
	headerKeepAlive       = "Keep-Alive"
	HeaderProxyConnection = "Proxy-Connection"
)

// 鉴权类
const (
	HeaderAuthorization      = "Authorization"
	HeaderProxyAuthenticate  = "Proxy-Authenticate"
	HeaderProxyAuthorization = "Proxy-Authorization"
	HeaderWWWAuthenticate    = "WWW-Authenticate"
)

// 区间请求类
const (
	HeaderAcceptRanges = "Accept-Ranges"
	HeaderContentRange = "Content-Range"
	HeaderIfRange      = "If-Range"
	HeaderRange        = "Range"
)

// 响应上下文类
const (
	HeaderAllow       = "Allow"
	HeaderServer      = "Server"
	HeaderServerLower = "server"
)

// 请求上下文
const (
	HeaderFrom          = "From"
	HeaderHost          = "Host"
	HeaderReferer       = "Referer"
	HeaderRefererPolicy = "Referer-Policy"
	HeaderUserAgent     = "User-Agent"
)

// 正文信息
const (
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentLanguage = "Content-Language"
	HeaderContentLength   = "Content-Length"
	HeaderContentLocation = "Content-Location"
	HeaderContentType     = "Content-Type"
)

// 内容协商类
const (
	HeaderAccept         = "Accept"
	HeaderAcceptCharset  = "Accept-Charset"
	HeaderAcceptEncoding = "Accept-Encoding"
	HeaderAcceptLanguage = "Accept-Language"
	HeaderAltSvc         = "Alt-Svc"
)

// 协议类
const (
	HTTP11 = "HTTP/1.1"
	HTTP10 = "HTTP/1.0"
	HTTP20 = "HTTP/2.0"
)

// 文本类 MIME
const (
	MIMETextPlain             = "text/plain"
	MIMETextPlainUTF8         = "text/plain; charset=utf-8"
	MIMETextPlainISO88591     = "text/plain; charset=iso-8859-1"
	MIMETextPlainFormatFlowed = "text/plain; format=flowed"
	MIMETextPlainDelSpaceYes  = "text/plain; delsp=yes"
	MiMETextPlainDelSpaceNo   = "text/plain; delsp=no"
	MIMETextHtml              = "text/html"
	MIMETextCss               = "text/css"
	MIMETextJavascript        = "text/javascript"
	MIMETextEventStream       = "text/event-stream"
)

// 应用类 MIME
const (
	MIMEApplicationOctetStream  = "application/octet-stream"
	MIMEApplicationFlash        = "application/x-shockwave-flash"
	MIMEApplicationHTMLForm     = "application/x-www-form-urlencoded"
	MIMEApplicationHTMLFormUTF8 = "application/x-www-form-urlencoded; charset=UTF-8"
	MIMEApplicationTar          = "application/x-tar"
	MIMEApplicationGZip         = "application/gzip"
	MIMEApplicationXGZip        = "application/x-gzip"
	MIMEApplicationBZip2        = "application/bzip2"
	MIMEApplicationXBZip2       = "application/x-bzip2"
	MIMEApplicationShell        = "application/x-sh"
	MIMEApplicationDownload     = "application/x-msdownload"
	MIMEApplicationJSON         = "application/json"
	MIMEApplicationJSONUTF8     = "application/json; charset=utf-8"
	MIMEApplicationXML          = "application/xml"
	MIMEApplicationXMLUTF8      = "application/xml; charset=utf-8"
	MIMEApplicationZip          = "application/zip"
	MIMEApplicationPdf          = "application/pdf"
	MIMEApplicationWord         = "application/msword"
	MIMEApplicationExcel        = "application/vnd.ms-excel"
	MIMEApplicationPPT          = "application/vnd.ms-powerpoint"
	MIMEApplicationOpenXMLWord  = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MIMEApplicationOpenXMLExcel = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	MIMEApplicationOpenXMLPPT   = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

// 图片类 MIME
const (
	MIMEImageJPEG         = "image/jpeg"
	MIMEImagePNG          = "image/png"
	MIMEImageGIF          = "image/gif"
	MIMEImageBitmap       = "image/bmp"
	MIMEImageWebP         = "image/webp"
	MIMEImageIco          = "image/x-icon"
	MIMEImageMicrosoftICO = "image/vnd.microsoft.icon"
	MIMEImageTIFF         = "image/tiff"
	MIMEImageSVG          = "image/svg+xml"
	MIMEImagePhotoshop    = "image/vnd.adobe.photoshop"
)

// 音频类 MIME
const (
	MIMEAudioBasic     = "audio/basic"
	MIMEAudioL24       = "audio/L24"
	MIMEAudioMP3       = "audio/mp3"
	MIMEAudioMP4       = "audio/mp4"
	MIMEAudioMPEG      = "audio/mpeg"
	MIMEAudioOggVorbis = "audio/ogg"
	MIMEAudioWAVE      = "audio/vnd.wave"
	MIMEAudioWebM      = "audio/webm"
	MIMEAudioAAC       = "audio/x-aac"
	MIMEAudioAIFF      = "audio/x-aiff"
	MIMEAudioMIDI      = "audio/x-midi"
	MIMEAudioM3U       = "audio/x-mpegurl"
	MIMEAudioRealAudio = "audio/x-pn-realaudio"
)

// 视频类 MIME
const (
	MIMEVideoMPEG          = "video/mpeg"
	MIMEVideoOgg           = "video/ogg"
	MIMEVideoMP4           = "video/mp4"
	MIMEVideoQuickTime     = "video/quicktime"
	MIMEVideoWinMediaVideo = "video/x-ms-wmv"
	MIMEVideWebM           = "video/webm"
	MIMEVideoFlashVideo    = "video/x-flv"
	MIMEVideo3GPP          = "video/3gpp"
	MIMEVideoAVI           = "video/x-msvideo"
	MIMEVideoMatroska      = "video/x-matroska"
)
