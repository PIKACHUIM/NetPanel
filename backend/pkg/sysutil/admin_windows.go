//go:build windows

package sysutil

import (
	"golang.org/x/sys/windows"
)

// IsAdmin 检测当前进程是否具有管理员权限（Windows）
// 通过检查当前进程令牌是否属于 Administrators 组来判断
func IsAdmin() bool {
	var sid *windows.SID
	// 构造 Administrators 组的 SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}
