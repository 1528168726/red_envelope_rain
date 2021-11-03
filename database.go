package main

import (
	"errors"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"time"
)

const mysqlUser = "group2"
const mysqlPassword = "Group2database"
const mysqlHost = "rdsmysqlh9eaae7ae6c1cb3e5.rds.volces.com"
const mysqlPort = "3306"
const mysqlDatabase = "rp_rain"

var Db *gorm.DB

type Envelopes struct {
	EnvelopeId int64 `gorm:"primaryKey"`
	Uid        int64
	Value      int
	Opened     bool
	SnatchTime int64
}

type Users struct {
	Uid      int64 `gorm:"primaryKey"`
	CurCount int
	ValueSum int64
}

type GlobalInfo struct {
	Id          int
	MaxReCount  int
	Probability float64
	Budget      int64
	Expenses    int64
}

func (v Envelopes) TableName() string {
	return "envelopes"
}

func (v Users) TableName() string {
	return "users"
}

func (v GlobalInfo) TableName() string {
	return "global_info"
}

func init() {
	//连接数据库
	err := connectToMySql()
	if err != nil {
		log.Print(err)
		os.Exit(0)
	}
	//初始化全局参数
	if Db.Where("id=1").First(&globalInfo).Error != nil {
		os.Exit(0)
	}
}

func connectToMySql() error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDatabase)
	Db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	return err
}

func getUser(uid int64) Users {
	res := Users{}
	Db.Where("uid=?", uid).First(&res)
	return res
}

func getUserCurCount(uid int64) int {
	res := Users{}
	err := Db.Select("cur_count").Where("uid=?", uid).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0
	}
	return res.CurCount
}

//未保证事务性
func insertEnvelopes(uid int64, value int, curCount int) (int64, error) {
	envelop := Envelopes{
		Uid:        uid,
		Value:      value,
		Opened:     false,
		SnatchTime: time.Now().Unix(),
	}
	err := Db.Create(&envelop).Error
	if err != nil {
		return 0, err
	}

	if curCount == 1 {
		user := Users{
			Uid:      uid,
			CurCount: 1,
			ValueSum: 0,
		}
		err = Db.Create(user).Error
		if err != nil {
			return 0, err
		}
	} else {
		err = Db.Model(&Users{}).Where("uid=?", uid).Update("cur_count", curCount).Error
		if err != nil {
			return 0, err
		}
	}

	return envelop.EnvelopeId, nil
}

//未保证事务性
func getEnvelopValue(envelopId int64) (int, bool, error) {
	var envelop Envelopes
	err := Db.First(&envelop, envelopId).Error
	if err != nil {
		return 0, false, err
	}
	opened := envelop.Opened
	if !opened {
		err = Db.Model(&envelop).Update("opened", true).Error
		if err != nil {
			return 0, false, err
		}
	}
	return envelop.Value, opened, nil
}

func updateUserValueSum(uid int64, value int) error {
	return Db.Model(&Users{}).Where("uid=?", uid).Update("value_sum", gorm.Expr("value_sum + ? ", value)).Error
}

func getEnvelopes(uid int64) ([]Envelopes, error) {
	var envelop []Envelopes
	err := Db.Where("uid=?", uid).Find(&envelop).Error
	if err != nil {
		return nil, err
	}
	return envelop, nil
}
