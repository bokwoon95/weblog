package pagemanager

import sq "github.com/bokwoon95/go-structured-query/postgres"

type table_pm_kv struct {
	*sq.TableInfo
	key   sq.StringField `ddl:"TEXT NOT NULL PRIMARY KEY"`
	value sq.StringField `ddl:"TEXT"`
}

func pm_kv() table_pm_kv {
	tbl := table_pm_kv{TableInfo: &sq.TableInfo{
		Name: "pm_kv",
	}}
	tbl.key = sq.NewStringField("key", tbl.TableInfo)
	tbl.value = sq.NewStringField("value", tbl.TableInfo)
	return tbl
}

func (tbl table_pm_kv) as(alias string) table_pm_kv {
	tbl.TableInfo.Alias = alias
	return tbl
}
