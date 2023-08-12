package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/favbox/wind/common/bytebufferpool"
	"github.com/favbox/wind/common/compress"
	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/internal/nocopy"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
)

var (
	errDirIndexRequired   = errors.NewPublic("目录索引不存在！")
	errNoCreatePermission = errors.NewPublic("没有创建文件权限！")

	rootFSOnce sync.Once
	rootFS     = &FS{
		Root:               "/",
		GenerateIndexPages: true,
		Compress:           true,
	}
	rootFSHandler HandlerFunc

	strInvalidHost = []byte("invalid-host")
)

// PathRewriteFunc 将请求路径改写为基于 FS.Root 的本地安全相对路径。
type PathRewriteFunc func(ctx *RequestContext) []byte

// FS 是静态文件服务配置项。 不支持拷贝。
type FS struct {
	noCopy nocopy.NoCopy

	// 静态文件服务的根目录。
	Root string

	// 访问目录时尝试打开的索引文件名称切片。
	//
	// 例如：
	//
	//	* index.html
	//	* index.htm
	//	* my-super-index.html
	//
	// 默认索引名称列表为空。
	IndexNames []string

	// 目录无 IndexNames 匹配文件时，要自动生成索引页?
	//
	// 多文件目录（超过 1K）生成索引页会很慢，故默认不开启。
	GenerateIndexPages bool

	// 是否压缩响应？
	//
	// 若启用压缩，能够最小化服务器的 CPU 用量。
	// 开启后将会添加 CompressedFileSuffix 后缀到原始文件名并另存为新文件。
	// 因此，开启前要授予根目录及所有子目录写权限。
	Compress bool

	// 要添加到缓存压缩文件名称的后缀。
	//
	// 仅在 Compress 开启时生效，默认值为 FSCompressedFileSuffix。
	CompressedFileSuffix string

	// 文件处理器的缓存时长。
	//
	// 默认值为 FSHandlerCacheDuration。
	CacheDuration time.Duration

	// 启用字节范围请求？
	//
	// 默认为禁用。
	AcceptByteRange bool

	// 路径重写函数。
	//
	// 默认不重写。
	PathRewrite PathRewriteFunc

	// 当文件不存在时可自定义处理方式。
	//
	// 默认返回 “无法打开请求路径”
	PathNotFound HandlerFunc

	once sync.Once
	h    HandlerFunc
}

// NewRequestHandler 返回当前 FS 的请求处理器。
//
// 返回的处理器会进行缓存，缓存时长为 FS.CacheDuration。
// 若 FS.Root 文件夹有大量文件，请通过 'ulimit -n' 提升文件打开数。
//
// 不要从单个 FS 实例创建多个请求处理器 - 只需复用一个请求处理器即可。
func (fs *FS) NewRequestHandler() HandlerFunc {
	fs.once.Do(fs.initRequestHandler)
	return fs.h
}

func (fs *FS) initRequestHandler() {
	root := fs.Root

	// 若根目录为空，则提供当前工作目录的文件服务
	if len(root) == 0 {
		root = "."
	}

	// 删除根路径的尾随斜线
	for len(root) > 0 && root[len(root)-1] == '/' {
		root = root[:len(root)-1]
	}

	// 设置文件的缓存时长
	cacheDuration := fs.CacheDuration
	if cacheDuration <= 0 {
		cacheDuration = consts.FSHandlerCacheDuration
	}

	// 设置压缩文件的后缀
	compressedFileSuffix := fs.CompressedFileSuffix
	if len(compressedFileSuffix) == 0 {
		compressedFileSuffix = consts.FSCompressedFileSuffix
	}

	h := &fsHandler{
		root:                 root,
		indexNames:           fs.IndexNames,
		pathRewrite:          fs.PathRewrite,
		pathNotFound:         fs.PathNotFound,
		generateIndexPages:   fs.GenerateIndexPages,
		compress:             fs.Compress,
		acceptByteRange:      fs.AcceptByteRange,
		cacheDuration:        cacheDuration,
		compressedFileSuffix: compressedFileSuffix,
		cache:                make(map[string]*fsFile),
		compressedCache:      make(map[string]*fsFile),
	}

	go func() {
		var pendingFiles []*fsFile
		for {
			time.Sleep(cacheDuration / 2)
			pendingFiles = h.cleanCache(pendingFiles)
		}
	}()

	fs.h = h.handleRequest
}

type fsHandler struct {
	root                 string
	indexNames           []string
	pathRewrite          PathRewriteFunc
	pathNotFound         HandlerFunc
	generateIndexPages   bool
	compress             bool
	acceptByteRange      bool
	cacheDuration        time.Duration
	compressedFileSuffix string

	cache           map[string]*fsFile
	compressedCache map[string]*fsFile
	cacheLock       sync.Mutex

	smallFileReaderPool sync.Pool
}

// 真正的静态文件服务处理器。
func (h *fsHandler) handleRequest(c context.Context, ctx *RequestContext) {
	var path []byte
	if h.pathRewrite != nil {
		path = h.pathRewrite(ctx)
	} else {
		path = ctx.Path()
	}
	path = stripTrailingSlashes(path)

	if n := bytes.IndexByte(path, 0); n >= 0 {
		wlog.SystemLogger().Errorf("无法提供空路径服务，位置=%d，路径=%q", n, path)
		ctx.AbortWithMsg("你是黑客吗？", consts.StatusBadRequest)
		return
	}

	if h.pathRewrite != nil {
		// 若 path 来自 ctx.Path() 则无需检查 '/../'，因为 ctx.Path() 已进行规范化。

		if n := bytes.Index(path, bytestr.StrSlashDotDotSlash); n >= 0 {
			wlog.SystemLogger().Errorf("由于安全原因，无法提供带有 '/../' 的路径服务，位置=%d，路径=%q", n, path)
			ctx.AbortWithMsg("内部服务器错误", consts.StatusInternalServerError)
			return
		}
	}

	// 是否需要压缩？
	mustCompress := false
	fileCache := h.cache
	byteRange := ctx.Request.Header.PeekRange()
	if len(byteRange) == 0 && h.compress && ctx.Request.Header.HasAcceptEncodingBytes(bytestr.StrGzip) {
		mustCompress = true
		fileCache = h.compressedCache
	}

	// 从缓存读取请求的文件
	h.cacheLock.Lock()
	ff, ok := fileCache[string(path)]
	if ok {
		ff.readersCount++
	}
	h.cacheLock.Unlock()

	// 读取请求的 fsFile，并缓存
	if !ok {
		pathStr := string(path)
		filePath := h.root + pathStr
		var err error
		ff, err = h.openFSFile(filePath, mustCompress)

		if mustCompress && err == errNoCreatePermission {
			wlog.SystemLogger().Errorf("权限不足，无法保存压缩文件 %q。正在提供未压缩文件。"+
				"授予该文件所在目录的写权限，可提高服务器性能。", filePath)
			mustCompress = false
			ff, err = h.openFSFile(filePath, mustCompress)
		}
		if err == errDirIndexRequired {
			ff, err = h.openIndexFile(ctx, filePath, mustCompress)
			if err != nil {
				wlog.SystemLogger().Errorf("无法打开目录索引文件，路径=%q, 错误=%s", filePath, err)
				ctx.AbortWithMsg("目录索引被禁止", consts.StatusForbidden)
				return
			}
		} else if err != nil {
			wlog.SystemLogger().Errorf("无法打开文件，路径=%q, 错误=%s", filePath, err)
			if h.pathNotFound == nil {
				ctx.AbortWithMsg("无法打开请求的路径", consts.StatusNotFound)
			} else {
				ctx.SetStatusCode(consts.StatusNotFound)
				h.pathNotFound(c, ctx)
			}
			return
		}

		h.cacheLock.Lock()
		ff1, ok := fileCache[pathStr]
		if !ok {
			fileCache[pathStr] = ff
			ff.readersCount++
		} else {
			ff1.readersCount++
		}
		h.cacheLock.Unlock()

		if ok {
			// 文件已被其他协程打开，故关闭当前文件并改用由其他协程打开的文件
			ff.Release()
			ff = ff1
		}
	}

	// 内容未修改，直接返回
	if !ctx.IfModifiedSince(ff.lastModified) {
		ff.decReadersCount()
		ctx.NotModified()
		return
	}

	// 内容已修改，读取原始文件
	r, err := ff.NewReader()
	if err != nil {
		wlog.SystemLogger().Errorf("无法获取文件读取器，路径=%q，错误=%s", path, err)
		ctx.AbortWithMsg("内部服务器错误", consts.StatusInternalServerError)
		return
	}

	// 按需设置内容编码为 gzip
	hdr := &ctx.Response.Header
	if ff.compressed {
		hdr.SetContentEncodingBytes(bytestr.StrGzip)
	}

	// 按需设置按字节区间传输，以及相关状态码和内容长度
	statusCode := consts.StatusOK
	contentLength := ff.contentLength
	if h.acceptByteRange {
		hdr.SetCanonical(bytestr.StrAcceptRanges, bytestr.StrBytes)
		if len(byteRange) > 0 {
			startPos, endPos, err := ParseByteRange(byteRange, contentLength)
			if err != nil {
				r.(io.Closer).Close()
				wlog.SystemLogger().Errorf("无法解析字节区间 %q，路径=%q，错误=%s", byteRange, path, err)
				ctx.AbortWithMsg("无法处理所请求的数据区间，可能不在文件范围之内", consts.StatusRequestedRangeNotSatisfiable)
				return
			}

			if err = r.(byteRangeUpdater).UpdateByteRange(startPos, endPos); err != nil {
				r.(io.Closer).Close()
				wlog.SystemLogger().Errorf("无法更新字节区间 %q，路径=%q，错误=%s", byteRange, path, err)
				ctx.AbortWithMsg("内部服务器错误", consts.StatusInternalServerError)
				return
			}

			hdr.SetContentRange(startPos, endPos, contentLength)
			contentLength = endPos - startPos + 1
			statusCode = consts.StatusPartialContent
		}
	}

	// 设置内容修改时间并发送正文流
	hdr.SetCanonical(bytestr.StrLastModified, ff.lastModifiedStr)
	if !ctx.IsHead() {
		ctx.SetBodyStream(r, contentLength)
	} else {
		ctx.Response.ResetBody()
		ctx.Response.SkipBody = true
		ctx.Response.Header.SetContentLength(contentLength)
		if rc, ok := r.(io.Closer); ok {
			if err := rc.Close(); err != nil {
				wlog.SystemLogger().Errorf("无法关闭文件读取器：错误=%s", err)
				ctx.AbortWithMsg("内部服务器错误", consts.StatusInternalServerError)
				return
			}
		}
	}

	// 设置内容类型和状态码
	hdr.SetNoDefaultContentType(true)
	if len(hdr.ContentType()) == 0 {
		ctx.SetContentType(ff.contentType)
	}
	ctx.SetStatusCode(statusCode)
}

func (h *fsHandler) cleanCache(pendingFiles []*fsFile) []*fsFile {
	var filesToRelease []*fsFile

	h.cacheLock.Lock()

	// 关闭之前由于读取器计数非零而无法关闭的文件
	var remainingFiles []*fsFile
	for _, ff := range pendingFiles {
		if ff.readersCount > 0 {
			remainingFiles = append(remainingFiles, ff)
		} else {
			filesToRelease = append(filesToRelease, ff)
		}
	}
	pendingFiles = remainingFiles

	pendingFiles, filesToRelease = cleanCacheNoLock(h.cache, pendingFiles, filesToRelease, h.cacheDuration)
	pendingFiles, filesToRelease = cleanCacheNoLock(h.compressedCache, pendingFiles, filesToRelease, h.cacheDuration)

	h.cacheLock.Unlock()

	for _, ff := range filesToRelease {
		ff.Release()
	}

	return pendingFiles
}

func cleanCacheNoLock(cache map[string]*fsFile, pendingFiles, filesToRelease []*fsFile, cacheDuration time.Duration) ([]*fsFile, []*fsFile) {
	t := time.Now()
	for k, ff := range cache {
		if t.Sub(ff.t) > cacheDuration {
			if ff.readersCount > 0 {
				// 过期文件上有挂起的读取器，所以还不能关闭。
				// 将其放入挂起文件，以便稍后关闭。
				pendingFiles = append(pendingFiles, ff)
			} else {
				filesToRelease = append(filesToRelease, ff)
			}
			delete(cache, k)
		}
	}
	return pendingFiles, filesToRelease
}

func (h *fsHandler) compressAndOpenFSFile(filePath string) (*fsFile, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("无法获取文件信息 %q: %s", filePath, err)
	}

	if fileInfo.IsDir() {
		f.Close()
		return nil, errDirIndexRequired
	}

	// 无需压缩的文件，直接返回
	if strings.HasSuffix(filePath, h.compressedFileSuffix) || // 已经压缩了
		fileInfo.Size() > consts.FsMaxCompressibleFileSize || // 大于 8MB
		!isFileCompressible(f, consts.FSMinCompressRatio) { // 压缩率不高
		return h.newFSFile(f, fileInfo, false)
	}

	compressedFilePath := filePath + h.compressedFileSuffix
	absPath, err := filepath.Abs(compressedFilePath)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("无法确定其绝对路径 %q: %s", compressedFilePath, err)
	}

	flock := getFileLock(absPath)
	flock.Lock()
	ff, err := h.compressFileNolock(f, fileInfo, filePath, compressedFilePath)
	flock.Unlock()

	return ff, err
}

func (h *fsHandler) compressFileNolock(f *os.File, fileInfo os.FileInfo, filePath, compressedFilePath string) (*fsFile, error) {
	// 尝试打开由其他并发协程创建的压缩文件。
	// 该做法是安全的，因为文件创建受文件互斥锁保护 —— 见 getFileLock 调用。
	if _, err := os.Stat(compressedFilePath); err == nil {
		f.Close()
		return h.newCompressedFSFile(compressedFilePath)
	}

	// 创建临时文件，所以并发协程在创建之前不会使用它。
	tmpFilePath := compressedFilePath + ".tmp"
	zf, err := os.Create(tmpFilePath)
	if err != nil {
		f.Close()
		if !os.IsPermission(err) {
			return nil, fmt.Errorf("无法创建临时文件 %q: %s", tmpFilePath, err)
		}
		return nil, errNoCreatePermission
	}

	zw := compress.AcquireStacklessGzipWriter(zf, compress.CompressDefaultCompression)
	zrw := network.NewWriter(zw)
	_, err = utils.CopyZeroAlloc(zrw, f)
	if err1 := zw.Flush(); err == nil {
		err = err1
	}
	compress.ReleaseStacklessGzipWriter(zw, compress.CompressDefaultCompression)
	zf.Close()
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("压缩文件发生错误 %q: %s", filePath, err)
	}
	if err = os.Chtimes(tmpFilePath, time.Now(), fileInfo.ModTime()); err != nil {
		return nil, fmt.Errorf("无法更改临时文件 %q 的修改时间为 %s: %s", tmpFilePath, fileInfo.ModTime(), err)
	}
	if err = os.Rename(tmpFilePath, compressedFilePath); err != nil {
		return nil, fmt.Errorf("无法移动压缩文件 %q 到 %q: %s", tmpFilePath, compressedFilePath, err)
	}
	return h.newCompressedFSFile(compressedFilePath)
}

// ParseByteRange 解析标头 'Range: bytes=...' 的值。
func ParseByteRange(byteRange []byte, contentLength int) (startPos, endPos int, err error) {
	b := byteRange
	if !bytes.HasPrefix(b, bytestr.StrBytes) {
		return 0, 0, fmt.Errorf("不支持的 Range 单位: %q，期望 %q", byteRange, bytestr.StrBytes)
	}

	b = b[len(bytestr.StrBytes):]
	if len(b) == 0 || b[0] != '=' {
		return 0, 0, fmt.Errorf("缺少字节区间的值：%q", byteRange)
	}
	b = b[1:]

	n := bytes.IndexByte(b, '-')
	if n < 0 {
		return 0, 0, fmt.Errorf("缺少字节区间的结束位置：%q", byteRange)
	}

	if n == 0 {
		v, err := bytesconv.ParseUint(b[n+1:])
		if err != nil {
			return 0, 0, err
		}
		startPos = contentLength - v
		if startPos < 0 {
			startPos = 0
		}
		return startPos, contentLength - 1, nil
	}

	if startPos, err = bytesconv.ParseUint(b[:n]); err != nil {
		return 0, 0, err
	}
	if startPos >= contentLength {
		return 0, 0, fmt.Errorf("字节区间的起始位置不能超过 %d。字节区间：%q", contentLength-1, byteRange)
	}

	b = b[n+1:]
	if len(b) == 0 {
		return startPos, contentLength - 1, err
	}

	if endPos, err = bytesconv.ParseUint(b); err != nil {
		return 0, 0, err
	}
	if endPos >= contentLength {
		endPos = contentLength - 1
	}
	if endPos < startPos {
		return 0, 0, fmt.Errorf("字节区间的起始位置不能超过结束位置。字节区间：%q", byteRange)
	}

	return startPos, endPos, nil
}

func (h *fsHandler) openFSFile(filePath string, mustCompress bool) (*fsFile, error) {
	filePathOriginal := filePath
	if mustCompress {
		filePath += h.compressedFileSuffix
	}

	f, err := os.Open(filePath)
	if err != nil {
		// 压缩文件不存在
		if mustCompress && os.IsNotExist(err) {
			return h.compressAndOpenFSFile(filePathOriginal)
		}
		return nil, err
	}

	fileInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("无法获取文件信息 %q: %s", filePath, err)
	}

	if fileInfo.IsDir() {
		f.Close()
		if mustCompress {
			return nil, fmt.Errorf("目录后缀异常：%q。后缀：%q", filePath, h.compressedFileSuffix)
		}
		return nil, errDirIndexRequired
	}

	if mustCompress {
		fileInfoOriginal, err := os.Stat(filePathOriginal)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("无法获取原始文件信息 %q: %s", filePathOriginal, err)
		}

		if fileInfoOriginal.ModTime() != fileInfo.ModTime() {
			// 压缩文件已过时。重新创建。
			f.Close()
			os.Remove(filePath)
			return h.compressAndOpenFSFile(filePathOriginal)
		}
	}

	return h.newFSFile(f, fileInfo, mustCompress)
}

var (
	filesLockMap     = make(map[string]*sync.Mutex)
	filesLockMapLock sync.Mutex
)

func getFileLock(absPath string) *sync.Mutex {
	filesLockMapLock.Lock()
	flock := filesLockMap[absPath]
	if flock == nil {
		flock = &sync.Mutex{}
		filesLockMap[absPath] = flock
	}
	filesLockMapLock.Unlock()
	return flock
}

func (h *fsHandler) createDirIndex(base *protocol.URI, dirPath string, mustCompress bool) (*fsFile, error) {
	w := &bytebufferpool.ByteBuffer{}

	basePathEscaped := html.EscapeString(string(base.Path()))
	fmt.Fprintf(w, "<html><head><title>%s</title><style>.dir {font-weight: bold}</style></head><body>", basePathEscaped)
	fmt.Fprintf(w, "<h1>%s</h1>", basePathEscaped)
	fmt.Fprintf(w, "<ul>")

	// 写入父级路径锚链接
	if len(basePathEscaped) > 1 {
		var parentURI protocol.URI
		base.CopyTo(&parentURI)
		parentURI.Update(string(base.Path()) + "/..")
		parentPathEscaped := html.EscapeString(string(parentURI.Path()))
		fmt.Fprintf(w, `<li><a href="%s" class="dir">..</a></li>`, parentPathEscaped)
	}

	f, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}

	fileInfos, err := f.Readdir(0)
	f.Close()
	if err != nil {
		return nil, err
	}

	fm := make(map[string]os.FileInfo, len(fileInfos))
	fileNames := make([]string, 0, len(fileInfos))
	for _, fi := range fileInfos {
		name := fi.Name()
		if strings.HasSuffix(name, h.compressedFileSuffix) {
			// 不在索引页显示缓存压缩文件
			continue
		}
		fm[name] = fi
		fileNames = append(fileNames, name)
	}

	var u protocol.URI
	base.CopyTo(&u)
	u.Update(string(u.Path()) + "/")

	sort.Strings(fileNames)
	for _, name := range fileNames {
		u.Update(name)
		pathEscaped := html.EscapeString(string(u.Path()))
		fi := fm[name]
		auxStr := "目录"
		className := "dir"
		if !fi.IsDir() {
			auxStr = fmt.Sprintf("文件，%d 字节", fi.Size())
			className = "file"
		}
		fmt.Fprintf(w, `<li><a href="%s" class="%s">%s</a>，%s，最后修改时间 %s</li>`,
			pathEscaped, className, html.EscapeString(name), auxStr, fsModTime(fi.ModTime()))
	}

	fmt.Fprintf(w, "</ul></body></html>")
	if mustCompress {
		var zBuf bytebufferpool.ByteBuffer
		zBuf.B = compress.AppendGzipBytesLevel(zBuf.B, w.B, compress.CompressDefaultCompression)
		w = &zBuf
	}

	dirIndex := w.B
	lastModified := time.Now()
	ff := &fsFile{
		h:               h,
		dirIndex:        dirIndex,
		contentType:     "text/html; charset=utf-8",
		contentLength:   len(dirIndex),
		compressed:      mustCompress,
		lastModified:    lastModified,
		lastModifiedStr: bytesconv.AppendHTTPDate(make([]byte, 0, len(http.TimeFormat)), lastModified),
		t:               lastModified,
	}
	return ff, nil
}

func (h *fsHandler) openIndexFile(ctx *RequestContext, dirPath string, mustCompress bool) (*fsFile, error) {
	for _, indexName := range h.indexNames {
		indexFilePath := dirPath + "/" + indexName
		ff, err := h.openFSFile(indexFilePath, mustCompress)
		if err == nil {
			return ff, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("无法打开文件 %q: %s", indexFilePath, err)
		}
	}

	if !h.generateIndexPages {
		return nil, fmt.Errorf("无法访问没有索引页的目录。目录 %q", dirPath)
	}

	return h.createDirIndex(ctx.URI(), dirPath, mustCompress)
}

func (h *fsHandler) newFSFile(f *os.File, fileInfo os.FileInfo, compressed bool) (*fsFile, error) {
	n := fileInfo.Size()
	contentLength := int(n)
	if n != int64(contentLength) {
		f.Close()
		return nil, fmt.Errorf("文件过大：%d 字节", n)
	}

	// 检查内容类型
	ext := fileExtension(fileInfo.Name(), compressed, h.compressedFileSuffix)
	contentType := mime.TypeByExtension(ext)
	if len(contentType) == 0 {
		data, err := readFileHeader(f, compressed)
		if err != nil {
			return nil, fmt.Errorf("无法读取文件头 %q: %s", f.Name(), err)
		}
		contentType = http.DetectContentType(data)
	}

	lastModified := fileInfo.ModTime()
	ff := &fsFile{
		h:               h,
		f:               f,
		contentType:     contentType,
		contentLength:   contentLength,
		compressed:      compressed,
		lastModified:    lastModified,
		lastModifiedStr: bytesconv.AppendHTTPDate(make([]byte, 0, len(http.TimeFormat)), lastModified),
		t:               time.Now(),
	}
	return ff, nil
}

func (h *fsHandler) newCompressedFSFile(filePath string) (*fsFile, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开压缩文件 %q: %s", filePath, err)
	}
	fileInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("无法获取压缩文件的信息 %q: %s", filePath, err)
	}
	return h.newFSFile(f, fileInfo, true)
}

func fsModTime(t time.Time) any {
	return t.In(time.UTC).Truncate(time.Second)
}

func readFileHeader(f *os.File, compressed bool) ([]byte, error) {
	r := io.Reader(f)
	var zr *gzip.Reader
	if compressed {
		var err error
		if zr, err = compress.AcquireGzipReader(f); err != nil {
			return nil, err
		}
		r = zr
	}

	lr := &io.LimitedReader{
		R: r,
		N: 512,
	}
	data, err := io.ReadAll(lr)
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	if zr != nil {
		compress.ReleaseGzipReader(zr)
	}

	return data, err
}

func fileExtension(path string, compressed bool, compressedFileSuffix string) string {
	if compressed && strings.HasSuffix(path, compressedFileSuffix) {
		path = path[:len(path)-len(compressedFileSuffix)]
	}
	n := strings.LastIndexByte(path, '.')
	if n < 0 {
		return ""
	}
	return path[n:]
}

func isFileCompressible(f *os.File, minCompressRatio float64) bool {
	// 先尝试压缩文件的前 4kb 来确认压缩率能否超过给定的最小压缩率
	b := bytebufferpool.Get()
	zw := compress.AcquireStacklessGzipWriter(b, compress.CompressDefaultCompression)
	lr := &io.LimitedReader{
		R: f,
		N: 4096,
	}
	zrw := network.NewWriter(zw)
	_, err := utils.CopyZeroAlloc(zrw, lr)
	compress.ReleaseStacklessGzipWriter(zw, compress.CompressDefaultCompression)
	f.Seek(0, 0)
	if err != nil {
		return false
	}

	n := 4096 - lr.N
	zn := len(b.B)
	bytebufferpool.Put(b)
	compressible := float64(zn) < float64(n)*minCompressRatio
	return compressible
}

type fsFile struct {
	h             *fsHandler
	f             *os.File
	dirIndex      []byte
	contentType   string
	contentLength int
	compressed    bool

	lastModified    time.Time
	lastModifiedStr []byte

	t            time.Time
	readersCount int

	bigFiles     []*fsBigFileReader
	bigFilesLock sync.Mutex
}

func (ff *fsFile) NewReader() (io.Reader, error) {
	if ff.isBig() {
		r, err := ff.bigFileReader()
		if err != nil {
			ff.decReadersCount()
		}
		return r, err
	}
	return ff.smallFileReader(), nil
}

func (ff *fsFile) Release() {
	if ff.f != nil {
		ff.f.Close()

		if ff.isBig() {
			ff.bigFilesLock.Lock()
			for _, r := range ff.bigFiles {
				r.f.Close()
			}
			ff.bigFilesLock.Unlock()
		}
	}
}

func (ff *fsFile) isBig() bool {
	return ff.contentLength > consts.MaxSmallFileSize && len(ff.dirIndex) == 0
}

func (ff *fsFile) bigFileReader() (io.Reader, error) {
	if ff.f == nil {
		panic("BUG: ff.f 不能为空")
	}

	var r io.Reader

	ff.bigFilesLock.Lock()
	n := len(ff.bigFiles)
	if n > 0 {
		r = ff.bigFiles[n-1]
		ff.bigFiles = ff.bigFiles[:n-1]
	}
	ff.bigFilesLock.Unlock()

	if r != nil {
		return r, nil
	}

	f, err := os.Open(ff.f.Name())
	if err != nil {
		return nil, fmt.Errorf("无法打开已打开的文件：%s", err)
	}

	return &fsBigFileReader{
		f:  f,
		ff: ff,
		r:  f,
	}, nil
}

func (ff *fsFile) smallFileReader() io.Reader {
	v := ff.h.smallFileReaderPool.Get()
	if v == nil {
		v = &fsSmallFileReader{}
	}
	r := v.(*fsSmallFileReader)
	r.ff = ff
	r.endPos = ff.contentLength
	if r.startPos > 0 {
		panic("BUG: 发现了 startPos 非空的 fsSmallFileReader")
	}
	return r
}

func (ff *fsFile) decReadersCount() {
	ff.h.cacheLock.Lock()
	defer ff.h.cacheLock.Unlock()
	ff.readersCount--
	if ff.readersCount < 0 {
		panic("BUG: fsFile.readersCount 为负数！")
	}
}

type byteRangeUpdater interface {
	UpdateByteRange(startPos, endPos int) error
}

type fsBigFileReader struct {
	f  *os.File
	ff *fsFile
	r  io.Reader
	lr io.LimitedReader
}

func (r *fsBigFileReader) UpdateByteRange(startPos, endPos int) error {
	if _, err := r.f.Seek(int64(startPos), 0); err != nil {
		return err
	}
	r.r = &r.lr
	r.lr.R = r.f
	r.lr.N = int64(endPos - startPos + 1)
	return nil
}

func (r *fsBigFileReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *fsBigFileReader) WriteTo(w io.Writer) (n int64, err error) {
	if rf, ok := w.(io.ReaderFrom); ok {
		// 快路径。Sendfile 一定被触发。
		return rf.ReadFrom(r.r)
	}
	zw := network.NewWriter(w)
	// 慢路径
	return utils.CopyZeroAlloc(zw, r.r)
}

func (r *fsBigFileReader) Close() error {
	r.r = r.f
	n, err := r.f.Seek(0, 0)
	if err == nil {
		if n != 0 {
			panic("BUG: File.Seek(0, 0) 返回 (non-zero, nil)")
		}

		ff := r.ff
		ff.bigFilesLock.Lock()
		ff.bigFiles = append(ff.bigFiles, r)
		ff.bigFilesLock.Unlock()
	} else {
		r.f.Close()
	}
	r.ff.decReadersCount()
	return err
}

type fsSmallFileReader struct {
	ff       *fsFile
	startPos int
	endPos   int
}

func (r *fsSmallFileReader) UpdateByteRange(startPos, endPos int) error {
	r.startPos = startPos
	r.endPos = endPos
	return nil
}

func (r *fsSmallFileReader) Read(p []byte) (int, error) {
	tailLen := r.endPos - r.startPos
	if tailLen <= 0 {
		return 0, io.EOF
	}
	if len(p) > tailLen {
		p = p[:tailLen]
	}

	ff := r.ff
	if ff.f != nil {
		n, err := ff.f.ReadAt(p, int64(r.startPos))
		r.startPos += n
		return n, err
	}

	n := copy(p, ff.dirIndex[r.startPos:])
	r.startPos += n
	return n, nil
}

func (r *fsSmallFileReader) WriteTo(w io.Writer) (int64, error) {
	ff := r.ff

	var n int
	var err error
	if ff.f == nil {
		n, err = w.Write(ff.dirIndex[r.startPos:r.endPos])
		return int64(n), err
	}

	if rf, ok := w.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	curPos := r.startPos
	bufV := utils.CopyBufPool.Get()
	buf := bufV.([]byte)
	for err == nil {
		tailLen := r.endPos - curPos
		if tailLen <= 0 {
			break
		}
		if len(buf) > tailLen {
			buf = buf[:tailLen]
		}
		n, err = ff.f.ReadAt(buf, int64(curPos))
		nw, errW := w.Write(buf[:n])
		curPos += nw
		if errW == nil && nw != n {
			panic("BUG: Write(p) 返回 (n, nil)，但 n != len(p)")
		}
		if err == nil {
			err = errW
		}
	}
	utils.CopyBufPool.Put(bufV)

	if err == io.EOF {
		err = nil
	}
	return int64(curPos - r.startPos), err
}

func (r *fsSmallFileReader) Close() error {
	ff := r.ff
	ff.decReadersCount()
	r.ff = nil
	r.startPos = 0
	r.endPos = 0
	ff.h.smallFileReaderPool.Put(r)
	return nil
}

func stripLeadingSlashes(path []byte, stripSlashes int) []byte {
	for stripSlashes > 0 && len(path) > 0 {
		if path[0] != '/' {
			panic("BUG: 路径必须以/开始")
		}
		n := bytes.IndexByte(path[1:], '/')
		if n < 0 {
			path = path[:0]
			break
		}
		path = path[n+1:]
		stripSlashes--
	}
	return path
}

func stripTrailingSlashes(path []byte) []byte {
	for len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}

// NewPathSlashesStripper 返回路径重写器，删除路径中的 slashesCount 个前导斜杠。
//
// 示例：
//
//   - slashesCount = 0, 原始路径："/foo/bar"，结果："/foo/bar"
//   - slashesCount = 1, 原始路径："/foo/bar"，结果："/bar"
//   - slashesCount = 2, 原始路径："/foo/bar"，结果：""
//
// 返回的路径重写器可用作 FS.PathRewrite。
func NewPathSlashesStripper(slashesCount int) PathRewriteFunc {
	return func(ctx *RequestContext) []byte {
		return stripLeadingSlashes(ctx.Path(), slashesCount)
	}
}

// NewVHostPathRewriter 返回路径重写器，删除路径中的 slashesCount 个前导斜杠，
// 并前置请求的主机到路径中，以简化静态文件的虚拟主机。
//
// 示例：
//
//   - host=foobar.com, slashesCount=0, 原始路径="/foo/bar"，结果："/foobar.com/foo/bar"
//   - host=img.abc.com, slashesCount=1, 原始路径="/images/123/456.jpg"，结果："/img.abc.com/123/456.jpg"
func NewVHostPathRewriter(slashesCount int) PathRewriteFunc {
	return func(ctx *RequestContext) []byte {
		path := stripLeadingSlashes(ctx.Path(), slashesCount)
		host := ctx.Host()
		if n := bytes.IndexByte(host, '/'); n >= 0 {
			host = nil
		}
		if len(host) == 0 {
			host = strInvalidHost
		}
		b := bytebufferpool.Get()
		b.B = append(b.B, '/')
		b.B = append(b.B, host...)
		b.B = append(b.B, path...)
		ctx.URI().SetPathBytes(b.B)
		bytebufferpool.Put(b)

		return ctx.Path()
	}
}

// ServeFile 将给定路径的文件内容压缩后写入 HTTP 响应。
//
// 以下情况，HTTP 响应可能会包含未经压缩的文件内容：
//
//   - 缺少 'Accept-Encoding: gzip' 请求头。
//   - 无指定文件所在目录的写权限。
//
// 若指定路径为目录则返回目录的内容。
//
// 使用 ServeFileUncompressed 可提供未压缩的文件内容。
func ServeFile(ctx *RequestContext, path string) {
	rootFSOnce.Do(func() {
		rootFSHandler = rootFS.NewRequestHandler()
	})
	if len(path) == 0 || path[0] != '/' {
		// 将相对路径扩展为绝对路径
		var err error
		if path, err = filepath.Abs(path); err != nil {
			wlog.SystemLogger().Errorf("无法将相对路径 %q 解析为绝对路径，错误=%s", path, err)
			ctx.AbortWithMsg("内部服务器错误", consts.StatusInternalServerError)
			return
		}
	}
	ctx.Request.SetRequestURI(path)
	rootFSHandler(context.Background(), ctx)
}

// ServeFileUncompressed 将指定路径的文件无压缩的写入 HTTP 响应。
//
// 若指定路径为目录则返回目录的内容。
//
// 使用 ServeFile 可提供压缩的文件内容，以节省网络流量。
func ServeFileUncompressed(ctx *RequestContext, path string) {
	ctx.Request.Header.DelBytes(bytestr.StrAcceptEncoding)
	ServeFile(ctx, path)
}
