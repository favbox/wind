package config

import "time"

// 请求项预定义的一组配置方法。
var preDefinedOpts []RequestOption

// RequestOption 是配置请求项的唯一结构体。
type RequestOption struct {
	F func(o *RequestOptions)
}

// RequestOptions 是请求项结构体。
type RequestOptions struct {
	tags map[string]string
	isSD bool

	dialTimeout    time.Duration
	readTimeout    time.Duration
	writeTimeout   time.Duration
	requestTimeout time.Duration // 一般由 DoDeadline 或 DoTimeout 设定
	start          time.Time
}

// Apply 将指定的一组配置方法 opts 应用到请求配置项上。
func (o *RequestOptions) Apply(opts []RequestOption) {
	for _, opt := range opts {
		opt.F(o)
	}
}

// CopyTo 将当前请求配置项拷贝到目标 dst。
func (o *RequestOptions) CopyTo(dst *RequestOptions) {
	if dst.tags == nil {
		dst.tags = make(map[string]string)
	}

	for k, v := range o.tags {
		dst.tags[k] = v
	}

	dst.isSD = o.isSD
	dst.readTimeout = o.readTimeout
	dst.writeTimeout = o.writeTimeout
	dst.dialTimeout = o.dialTimeout
	dst.requestTimeout = o.requestTimeout
	dst.start = o.start
}

func (o *RequestOptions) IsSD() bool {
	return o.isSD
}

// DialTimeout 返回请求的拨号超时时长。
func (o *RequestOptions) DialTimeout() time.Duration {
	return o.dialTimeout
}

// ReadTimeout 返回请求的读取超时时长。
func (o *RequestOptions) ReadTimeout() time.Duration {
	return o.readTimeout
}

// WriteTimeout 返回请求的写入超时时长。
func (o *RequestOptions) WriteTimeout() time.Duration {
	return o.writeTimeout
}

// RequestTimeout 返回请求的超时时长。
func (o *RequestOptions) RequestTimeout() time.Duration {
	return o.requestTimeout
}

// StartRequest 记录请求的开始时间。
//
// 注意：框架自动调用，无需人工调用。
func (o *RequestOptions) StartRequest() {
	if o.requestTimeout > 0 {
		o.start = time.Now()
	}
}

// StartTime 返回请求的开始时间。
func (o *RequestOptions) StartTime() time.Time {
	return o.start
}

// Tag 返回指定请求标签中指定 k 的值。
func (o *RequestOptions) Tag(k string) string {
	return o.tags[k]
}

// Tags 返回请求标签映射。
func (o *RequestOptions) Tags() map[string]string {
	return o.tags
}

// NewRequestOptions 创建请求配置项并应用指定的配置函数。
func NewRequestOptions(opts []RequestOption) *RequestOptions {
	options := &RequestOptions{
		tags: make(map[string]string),
		isSD: false,
	}
	if preDefinedOpts != nil {
		options.Apply(preDefinedOpts)
	}
	options.Apply(opts)
	return options
}

// SetPreDefinedOpts 为请求项预定义一组配置方法。
func SetPreDefinedOpts(opts ...RequestOption) {
	preDefinedOpts = nil
	preDefinedOpts = append(preDefinedOpts, opts...)
}

// WithDialTimeout 设置请求的拨号超时时长。
//
// 这是请求级配置，优先于客户端级别的配置。
//
// 注意：如果连接池中的连接数超过了最大连接数，并且需要在等待时建立连接，则此操作不会生效。
func WithDialTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.dialTimeout = t
	}}
}

// WithReadTimeout 设置请求的读超时时长。
//
// 这是请求级配置，优先于客户端级别的配置。
func WithReadTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.readTimeout = t
	}}
}

// WithWriteTimeout 设置请求的写超时时长。
//
// 这是请求级配置，优先于客户端级别的配置。
func WithWriteTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.writeTimeout = t
	}}
}

// WithRequestTimeout 设置整个请求的超时时长。
// 若超时，客户端会退出请求。
//
// 这是请求级配置，优先于客户端级别的配置。
func WithRequestTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.requestTimeout = t
	}}
}

// WithSD 设置请求选项中的 isSD。
func WithSD(b bool) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.isSD = b
	}}
}

// WithTag 设置请求选项中的标签映射。
func WithTag(k, v string) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.tags[k] = v
	}}
}
