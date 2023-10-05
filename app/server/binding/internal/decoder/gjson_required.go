//go:build gjson || !(amd64 && (linux || windows || darwin))

package decoder

import (
	"strings"

	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/tidwall/gjson"
)

func checkRequiredJSON(req *protocol.Request, tagInfo TagInfo) bool {
	if !tagInfo.Required {
		return true
	}
	ct := bytesconv.B2s(req.Header.ContentType())
	if utils.FilterContentType(ct) != consts.MIMEApplicationJSON {
		return false
	}
	result := gjson.GetBytes(req.Body(), tagInfo.JSONName)
	if !result.Exists() {
		idx := strings.LastIndex(tagInfo.JSONName, ".")
		if idx > 0 && !gjson.GetBytes(req.Body(), tagInfo.JSONName[:idx]).Exists() {
			return true
		}
		return false
	}
	return true
}
