package wechat

import (
	"errors"
	"fmt"
)

type APIError struct {
	Code     int
	Msg      string
	Endpoint string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("wechat api %s failed: errcode=%d errmsg=%s", e.Endpoint, e.Code, e.Msg)
}

func IsTokenInvalid(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.Code {
		case 40001, 40014, 42001, 61014:
			return true
		}
	}
	return false
}

func IsTransient(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.Code {
		case -1, 9402202, 45009:
			return true
		}
		return false
	}
	// Network and timeout errors are worth a retry for read-only calls.
	return err != nil
}

func FriendlyMessage(code int) string {
	switch code {
	case -1:
		return "微信系统繁忙，请稍后重试"
	case 40001, 40014, 42001:
		return "接口令牌失效，请重试（系统会自动刷新令牌）"
	case 40013:
		return "AppID 不合法"
	case 40125:
		return "AppSecret 无效，请检查系统配置"
	case 45009:
		return "接口调用超过频率/每日额度限制"
	case 48001:
		return "接口未授权，请检查第三方平台权限集或账号授权"
	case 61004:
		return "服务器出口 IP 不在第三方平台白名单内"
	case 61005:
		return "component_verify_ticket 已过期，请确认回调地址并重新触发票据推送"
	case 61006:
		return "component_verify_ticket 无效，请确认回调地址和 Token/AESKey 配置"
	case 61039:
		return "代码隐私检测尚未完成，请等待约 1 分钟后重试"
	case 61040:
		return "代码使用了未配置/未申请的隐私接口，请先完善隐私保护指引或 ext.json"
	case 85007:
		return "审核页面路径错误，请用页面列表中的路径"
	case 85008:
		return "小程序没有审核通过的类目，请先在小程序后台配置类目"
	case 85009:
		return "该小程序已有版本正在审核中"
	case 85013:
		return "ext_json 自定义配置无效"
	case 85014:
		return "模板编号无效，请先同步代码模板"
	case 85018:
		return "域名未在第三方平台的小程序服务器域名中登记"
	case 85019:
		return "没有可发布的审核版本"
	case 85020:
		return "审核状态不满足发布条件"
	case 85058:
		return "该小程序暂无审核记录"
	case 85023:
		return "审核项数量必须在 1-5 之间"
	case 85064:
		return "找不到对应的草稿或模板"
	case 85065:
		return "模板库已满"
	case 85077:
		return "审核类目已失效，请重新配置类目"
	case 85085:
		return "本月提审数量已达上限"
	case 85086:
		return "提审前尚未上传代码"
	case 85301, 85302, 85303:
		return "域名不合规或缺少 ICP 备案，修改未生效"
	case 86000:
		return "当前调用不是第三方平台代小程序调用，请检查令牌类型"
	case 86002:
		return "小程序尚未完成昵称、头像、简介等基础信息初始化"
	case 87012:
		return "该版本不可回退"
	case 87013:
		return "撤回审核次数已达上限（每日 5 次 / 每月 10 次）"
	}
	return ""
}
