//go:build (linux || windows || darwin) && amd64 && !gjson

package decoder

import (
	"strings"

	"github.com/bytedance/sonic"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
)

func checkRequiredJSON(req *protocol.Request, tagInfo TagInfo) bool {
	if !tagInfo.Required {
		return true
	}
	ct := bytesconv.B2s(req.Header.ContentType())
	if utils.FilterContentType(ct) != consts.MIMEApplicationJSON {
		return false
	}
	node, _ := sonic.Get(req.Body(), stringSliceForInterface(tagInfo.JSONName)...)
	if !node.Exists() {
		idx := strings.LastIndex(tagInfo.JSONName, ".")
		if idx > 0 {
			// 若为空须有一个有限的，否则对于必填项将返回 true
			node, _ = sonic.Get(req.Body(), stringSliceForInterface(tagInfo.JSONName[:idx])...)
			if !node.Exists() {
				return true
			}
		}
		return false
	}
	return true
}

func stringSliceForInterface(s string) (ret []any) {
	x := strings.Split(s, ".")
	for _, val := range x {
		ret = append(ret, val)
	}
	return
}
