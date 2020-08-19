package sql_factory

import (
	"database/sql"
	"fmt"
	"go.uber.org/zap"
	"goskeleton/app/core/event_manage"
	"goskeleton/app/global/my_errors"
	"goskeleton/app/global/variable"
	"goskeleton/app/utils/yml_config"
	"time"
)

var mysqlDriver *sql.DB
var sqlserverDriver *sql.DB

// 初始化数据库驱动
func initSqlDriver(sqlType string) *sql.DB {
	configFac := yml_config.CreateYamlFactory()
	var err error
	if sqlType == "mysql" {

		Host := configFac.GetString("Mysql.Host")
		Port := configFac.GetString("Mysql.Port")
		User := configFac.GetString("Mysql.User")
		Pass := configFac.GetString("Mysql.Pass")
		Charset := configFac.GetString("Mysql.Charset")
		DataBase := configFac.GetString("Mysql.DataBase")
		SetMaxIdleConns := configFac.GetInt("Mysql.SetMaxIdleConns")
		SetMaxOpenConns := configFac.GetInt("Mysql.SetMaxOpenConns")
		SetConnMaxLifetime := configFac.GetDuration("Mysql.SetConnMaxLifetime")
		SqlConnString := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=True&loc=Local&charset=%s", User, Pass, Host, Port, DataBase, Charset)
		//db, err := sql.Open("mysql", string(User)+":"+Pass+"@tcp("+Host+":"+Port+")/"+DataBase+"?parseTime=True&loc=Local&charset="+Charset)
		mysqlDriver, err = sql.Open("mysql", SqlConnString)
		if err != nil {
			variable.ZapLog.Error(my_errors.ErrorsDbSqlDriverInitFail, zap.Error(err))
			return nil
		}
		mysqlDriver.SetMaxIdleConns(SetMaxIdleConns)
		mysqlDriver.SetMaxOpenConns(SetMaxOpenConns)
		mysqlDriver.SetConnMaxLifetime(SetConnMaxLifetime * time.Second)
		// 将需要销毁的事件统一注册在事件管理器，由程序退出时统一销毁
		event_manage.CreateEventManageFactory().Set(variable.EventDestroyPrefix+"Mysql_DB", func(args ...interface{}) {
			_ = mysqlDriver.Close()
		})
		return mysqlDriver

	} else if sqlType == "sqlserver" || sqlType == "mssql" {
		Host := configFac.GetString("SqlServer.Host")
		Port := configFac.GetString("SqlServer.Port")
		DataBase := configFac.GetString("SqlServer.DataBase")
		User := configFac.GetString("SqlServer.User")
		Pass := configFac.GetString("SqlServer.Pass")
		SetMaxIdleConns := configFac.GetInt("SqlServer.SetMaxIdleConns")
		SetMaxOpenConns := configFac.GetInt("SqlServer.SetMaxOpenConns")
		SetConnMaxLifetime := configFac.GetDuration("SqlServer.SetConnMaxLifetime")
		SqlConnString := fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s", Host, Port, DataBase, User, Pass)
		sqlserverDriver, err = sql.Open("mssql", SqlConnString)
		if err != nil {
			variable.ZapLog.Error(my_errors.ErrorsDbSqlDriverInitFail + err.Error())
			return nil
		}
		sqlserverDriver.SetMaxIdleConns(SetMaxIdleConns)
		sqlserverDriver.SetMaxOpenConns(SetMaxOpenConns)
		sqlserverDriver.SetConnMaxLifetime(SetConnMaxLifetime * time.Second)
		// 将需要销毁的事件统一注册在事件管理器，由程序退出时统一销毁
		event_manage.CreateEventManageFactory().Set(variable.EventDestroyPrefix+"Sqlserver_DB", func(args ...interface{}) {
			_ = sqlserverDriver.Close()
		})
		return sqlserverDriver
	}
	return nil
}

// 从连接池获取一个连接
func GetOneSqlClient(sqlType string) *sql.DB {
	configFac := yml_config.CreateYamlFactory()
	maxRetryTimes := configFac.GetInt("SqlServer.PingFailRetryTimes")
	// ping 失败允许重试
	var dbDriver *sql.DB
	switch sqlType {
	case "mysql":
		if mysqlDriver == nil {
			dbDriver = initSqlDriver(sqlType)
		} else {
			dbDriver = mysqlDriver
		}
	case "sqlserver", "mssql":
		if sqlserverDriver == nil {
			dbDriver = initSqlDriver(sqlType)
		} else {
			dbDriver = sqlserverDriver
		}
	default:
		variable.ZapLog.Error(my_errors.ErrorsDbDriverNotExists + "，" + sqlType)
		return nil
	}
	for i := 1; i <= maxRetryTimes; i++ {
		if err := dbDriver.Ping(); err != nil { //  获取一个连接失败，进行重试
			dbDriver = initSqlDriver(sqlType)
			time.Sleep(time.Second * yml_config.CreateYamlFactory().GetDuration("Mysql.ReConnectInterval"))
			if i == maxRetryTimes {
				variable.ZapLog.Error("Mysql：" + my_errors.ErrorsDbGetConnFail)
				return nil
			}
		} else {
			break
		}
	}
	fmt.Printf("dbDriver地址：%p\n", dbDriver)
	return dbDriver
}