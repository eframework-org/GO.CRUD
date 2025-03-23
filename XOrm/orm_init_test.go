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
		name    string
		prefs   XPrefs.IBase
		wantErr bool
	}{
		{
			name: "single_db_test",
			prefs: XPrefs.New().Set("Orm/MySQL/myalias", XPrefs.New().
				Set(prefsOrmAddr, TestOrmAddr).
				Set(prefsOrmPool, 10).
				Set(prefsOrmConn, 100)),
			wantErr: false,
		},
		{
			name: "multiple_db_test",
			prefs: XPrefs.New().
				Set("Orm/MySQL/myalias1", XPrefs.New().
					Set(prefsOrmAddr, "root:123456@tcp(127.0.0.1:3306)/mysql?charset=utf8mb4").
					Set(prefsOrmPool, 10).
					Set(prefsOrmConn, 100)).
				Set("Orm/MySQL/myalias2", XPrefs.New().
					Set(prefsOrmAddr, "root:123456@tcp(127.0.0.1:3306)/information_schema?charset=utf8mb4").
					Set(prefsOrmPool, 20).
					Set(prefsOrmConn, 200)),
			wantErr: false,
		},
		{
			name: "invalid_config_test",
			prefs: XPrefs.New().
				Set("Orm/MySQL/myalias3", XPrefs.New().
					Set(prefsOrmAddr, "root:wrongpass@tcp(127.0.0.1:3306)/mysql?charset=utf8mb4").
					Set(prefsOrmPool, 10).
					Set(prefsOrmConn, 100)),
			wantErr: true,
		},
		{
			name:    "nil_config_test",
			prefs:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset database cache
			orm.ResetModelCache()

			if tt.prefs == nil {
				defer func() {
					if r := recover(); r == nil && tt.wantErr {
						t.Errorf("setup() expected error")
					}
				}()
				initOrm(tt.prefs)
				return
			}

			// For invalid config test, expect panic
			if tt.wantErr {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("setup() expected error")
					}
				}()
				initOrm(tt.prefs)
				return
			}

			// Normal config test
			initOrm(tt.prefs)

			// Test multiple database connections
			aliases := []string{}
			if tt.name == "single_db_test" {
				aliases = append(aliases, "myalias")
			} else if tt.name == "multiple_db_test" {
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
