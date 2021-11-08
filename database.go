package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"strconv"
	"time"
)

const (
	mysqlUser     = "group2"
	mysqlPassword = "Group2database"
	mysqlHost     = "rdsmysqlh9eaae7ae6c1cb3e5.rds.volces.com"
	mysqlPort     = "3306"
	mysqlDatabase = "rp_rain"
	redisAddr     = "redis-cn02db5alb97tbj4z.redis.volces.com:6379"
	redisPassword = "Group2database"
)

var Db *gorm.DB
var redis_client *redis.Client
var ctx = context.Background()

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

	//redis连接
	redis_client = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		PoolSize: 100,
	})
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

func getUserCurCount(uid int64) (count int) {
	userString := fmt.Sprintf("user_%d", uid)
	num, err := redis_client.HGet(ctx, userString, "cur_count").Result()
	// redis中不存在
	if err != nil {
		res := Users{}
		err = Db.Select("cur_count").Where("uid=?", uid).First(&res).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			go func() {
				user := Users{
					Uid:      uid,
					CurCount: 0,
					ValueSum: 0,
				}
				err = Db.Create(&user).Error
				if err != nil {
					log.Print(err)
				}
			}()
			redis_client.HSet(ctx, userString, "cur_count", 0)
			redis_client.HSet(ctx, userString, "balance", 0)
		}
		return 0
	}
	i, err := strconv.Atoi(num)
	if err != nil {
		log.Print("str conv 2 int error")
	}
	return i
}

//未保证事务性
func insertEnvelopes(uid int64, value int, curCount int) (int64, error) {
	userEnvString := fmt.Sprintf("user_%d_envList", uid)
	userString := fmt.Sprintf("user_%d", uid)
	now := time.Now().Unix()
	envChan := make(chan Envelopes, 0)
	// 写MySQL
	go func() {
		envelop := Envelopes{
			Uid:        uid,
			Value:      value,
			Opened:     false,
			SnatchTime: now,
		}
		err := Db.Create(&envelop).Error
		if err != nil {
			log.Print(err)
		}
		err = Db.Model(&Users{}).Where("uid=?", uid).Update("cur_count", curCount).Error
		if err != nil {
			log.Print(err)
		}
		envChan <- envelop
	}()
	env := <-envChan
	// 当前user拥有红包List
	redis_client.RPush(ctx, userEnvString, env.EnvelopeId)
	// 更新红包表
	envIdSrting := strconv.FormatInt(env.EnvelopeId, 10)
	redis_client.HSet(ctx, envIdSrting, "value", value)
	redis_client.HSet(ctx, envIdSrting, "opened", false)
	redis_client.HSet(ctx, envIdSrting, "time", now)

	// 更新redis user's cur_count
	redis_client.HIncrBy(ctx, userString, "cur_count", 1)

	return env.EnvelopeId, nil
}

//未保证事务性
func getEnvelopValue(envelopId int64) (int, bool, error) {
	envIdString := strconv.FormatInt(envelopId, 10)
	valueStr, err := redis_client.HGet(ctx, envIdString, "value").Result()
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Print("str 2 int error")
	}
	var opened bool
	//redis不存在 查SQL
	if err != nil {
		var envelop Envelopes
		err := Db.First(&envelop, envelopId).Error
		if err != nil {
			return 0, false, err
		}
		opened = envelop.Opened
		if !opened {
			err = Db.Model(&envelop).Update("opened", true).Error
			if err != nil {
				return 0, false, err
			}
		}
		redis_client.HSet(ctx, envIdString, "value", envelop.Value)
		redis_client.HSet(ctx, envIdString, "opened", true)
		redis_client.HSet(ctx, envIdString, "time", envelop.SnatchTime)
		return value, opened, nil
	}
	open, _ := redis_client.HGet(ctx, envIdString, "opened").Result()
	if open == "1" {
		opened = true
	} else {
		opened = false
	}
	return value, opened, nil
}

func updateUserValueSum(uid int64, value int) error {
	userString := fmt.Sprintf("user_%d", uid)
	redis_client.HIncrBy(ctx, userString, "balance", int64(value))
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
