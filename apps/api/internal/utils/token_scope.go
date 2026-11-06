package utils

// 令牌范围（scope）：同一套 JWT 承载两种权限面。
//
// 为什么需要它：账号因资料不合规被冻结后，登录不能直接 403——
// 那样用户拿不到令牌，就永远调不了补资料的 PUT /me，形成死锁。
// 但也不能直接发正常令牌，否则冻结形同虚设。
//
// 于是发一张「只能补资料」的受限令牌：能读自己、能改自己、能提交解冻，
// 其余接口一律 403。补齐资料后换发正常令牌。
const (
	// ScopeFull 完整权限：正常登录签发的令牌。
	ScopeFull = "full"
	// ScopeProfileOnly 仅补资料：冻结账号登录时签发。
	ScopeProfileOnly = "profile_only"
)

// IsValidScope 判断 scope 取值是否合法。
//
// 空串按 ScopeFull 处理：本次改动之前签发的旧令牌没有 scope claim，
// 它们都是完整权限的正常令牌，不能因为加了新字段就让所有人掉线。
func IsValidScope(scope string) bool {
	return scope == "" || scope == ScopeFull || scope == ScopeProfileOnly
}

// NormalizeScope 把空串归一到 ScopeFull（向后兼容旧令牌）。
func NormalizeScope(scope string) string {
	if scope == "" {
		return ScopeFull
	}
	return scope
}
