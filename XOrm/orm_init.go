// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"strings"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XPrefs"

	_ "github.com/go-sql-driver/mysql"
)

const (
	prefsOrmAddr = "Addr"
	prefsOrmPool = "Pool"
	prefsOrmConn = "Conn"
)

func init() {
	initOrm(XPrefs.Asset())
}

func initOrm(prefs XPrefs.IBase) {
	if prefs == nil {
		XLog.Panic("XOrm.Init: prefs is nil.")
		return
	}

	for _, key := range prefs.Keys() {
		if !strings.HasPrefix(key, "Orm/") {
			continue
		}
		parts := strings.Split(key, "/")
		if len(parts) < 3 {
			XLog.Panic("XOrm.Init: invalid prefs key %v.", key)
			return
		}

		ormType := strings.ToLower(parts[1])
		ormAlias := parts[2]

		if base := prefs.Get(key).(XPrefs.IBase); base != nil {
			ormAddr := base.GetString(prefsOrmAddr)
			ormPool := base.GetInt(prefsOrmPool)
			ormConn := base.GetInt(prefsOrmConn)
			if err := orm.RegisterDataBase(ormAlias, ormType, ormAddr,
				orm.MaxIdleConnections(ormPool),
				orm.MaxOpenConnections(ormConn)); err != nil {
				XLog.Panic("XOrm.Init: register database %v failed, err: %v", ormAlias, err)
				return
			}
		} else {
			XLog.Error("XOrm.Init: invalid config for %v", key)
			continue
		}
	}
}
