// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"testing"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XPrefs"
)

const (
	TestOrmAddr = "root:123456@tcp(127.0.0.1:3306)/mysql?charset=utf8mb4&loc=Local"
)

func TestOrmInit(t *testing.T) {
	tests := []struct {
		name  string
		prefs XPrefs.IBase
		panic bool
	}{
		{
			name: "Single",
			prefs: XPrefs.New().Set("Orm/Source/MySQL/myalias", XPrefs.New().
				Set(prefsOrmAddr, TestOrmAddr).
				Set(prefsOrmPool, 10).
				Set(prefsOrmConn, 100)),
			panic: false,
		},
		{
			name: "Multiple",
			prefs: XPrefs.New().
				Set("Orm/Source/MySQL/myalias1", XPrefs.New().
					Set(prefsOrmAddr, "root:123456@tcp(127.0.0.1:3306)/mysql?charset=utf8mb4").
					Set(prefsOrmPool, 10).
					Set(prefsOrmConn, 100)).
				Set("Orm/Source/MySQL/myalias2", XPrefs.New().
					Set(prefsOrmAddr, "root:123456@tcp(127.0.0.1:3306)/information_schema?charset=utf8mb4").
					Set(prefsOrmPool, 20).
					Set(prefsOrmConn, 200)),
			panic: false,
		},
		{
			name: "Invalid",
			prefs: XPrefs.New().
				Set("Orm/Source/MySQL/myalias3", XPrefs.New().
					Set(prefsOrmAddr, "root:wrongpass@tcp(127.0.0.1:3306)/mysql?charset=utf8mb4").
					Set(prefsOrmPool, 10).
					Set(prefsOrmConn, 100)),
			panic: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset database cache
			orm.ResetModelCache()

			if test.prefs == nil {
				defer func() {
					if r := recover(); r == nil && test.panic {
						t.Errorf("setup() expected error")
					}
				}()
				initOrm(test.prefs)
				return
			}

			// For invalid config test, expect panic
			if test.panic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("setup() expected error")
					}
				}()
				initOrm(test.prefs)
				return
			}

			// Normal config test
			initOrm(test.prefs)

			// Test multiple database connections
			aliases := []string{}
			switch test.name {
			case "single_db_test":
				aliases = append(aliases, "myalias")
			case "multiple_db_test":
				aliases = append(aliases, "myalias1", "myalias2")
			}

			for _, alias := range aliases {
				// Try to get database connection
				db, err := orm.GetDB(alias)
				if err != nil {
					t.Errorf("failed to get database connection [%s]: %v", alias, err)
					return
				}

				// Test database connection
				if err := db.Ping(); err != nil {
					t.Errorf("failed to ping database [%s]: %v", alias, err)
					return
				}
			}
		})
	}
}
