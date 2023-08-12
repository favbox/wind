package basic_auth

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol/consts"
)

// Accounts 用于构建用户名:密码映射。
type Accounts map[string]string

// 用于构建标头值:用户名的反向映射。
type pairs map[string]string

func (p pairs) findValue(needle string) (v string, ok bool) {
	v, ok = p[needle]
	return
}

func constructPairs(accounts Accounts) pairs {
	length := len(accounts)
	p := make(pairs, length)
	for user, password := range accounts {
		value := "Basic " + base64.StdEncoding.EncodeToString(bytesconv.S2b(user+":"+password))
		p[value] = user
	}
	return p
}

// BasicAuthForRealm 返回指定领域和用户标头键名的基本 HTTP 授权中间件。
// accounts 类型为 map[string]string，其中 key 是用户名，value 密码。
// realm 是资源所在的领域名称，若为空白字符串则默认使用 "Authorization Required"。
// 详见 http://tools.ietf.org/html/rfc2617#section-1.2
func BasicAuthForRealm(accounts Accounts, realm, userKey string) app.HandlerFunc {
	realm = "Basic realm=" + strconv.Quote(realm)
	p := constructPairs(accounts)
	return func(c context.Context, ctx *app.RequestContext) {
		// 在允许的凭据切片中搜索用户
		user, found := p.findValue(ctx.Request.Header.Get("Authorization"))
		if !found {
			//	凭据不匹配，返回401并终止处理链。
			ctx.Header("WWW-Authenticate", realm)
			ctx.AbortWithStatus(consts.StatusUnauthorized)
			return
		}

		// 找到用户凭证，以 userKey 为键存储在上下文中以供后续使用。
		ctx.Set(userKey, user)
	}
}

// BasicAuth 用于构造 Wind 授权中间件。
// 它返回一个中间件，以 map[string]string 为参数，其中 key 是用户名，value 密码。
func BasicAuth(accounts Accounts) app.HandlerFunc {
	return BasicAuthForRealm(accounts, "Authorization Required", "user")
}
