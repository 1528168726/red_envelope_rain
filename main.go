package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"log"
	"math/rand"
	"net/http"
	"time"
)

var globalInfo GlobalInfo

func main() {
	//
	// 创建路由
	r := gin.Default()
	// 绑定三个接口
	r.POST("/snatch", snatchFunc)
	r.POST("/open", openFunc)
	r.POST("/get_wallet_list", getWalletListFunc)
	err := r.Run(":8080")
	if err != nil {
		log.Print(err)
	}
}

type SnatchRequest struct {
	Uid int64 `json:"uid"`
}

type SnatchRespond struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		EnvelopeId int64 `json:"envelopeId"`
		MaxCount   int   `json:"maxCount"`
		CurCount   int   `json:"curCount"`
	} `json:"data"`
}

type OpenRequest struct {
	Uid        int64 `json:"uid"`
	EnvelopeId int64 `json:"envelope_id"`
}

type OpenRespond struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Value int `json:"value"`
	} `json:"data"`
}

type GetWalletListRequest struct {
	Uid int64 `json:"uid"`
}

type GetWalletListRespond struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Amount       int `json:"amount"`
		EnvelopeList []gin.H
	} `json:"data"`
}

func snatchFunc(c *gin.Context) {
	request := SnatchRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusOK, gin.H{
			"Code": JsonUnmarshalError,
			"Msg":  "解析错误",
		})
		return
	}
	//判断红包数量
	curRevelopCount := getUserCurCount(request.Uid)
	if curRevelopCount >= globalInfo.MaxReCount {
		c.JSON(http.StatusOK, gin.H{
			"Code": RevelopsCountExceed,
			"Msg":  "红包数量超出限制",
		})
		return
	}
	//生成随机数以发放红包
	rand.Seed(time.Now().Unix())
	randomNumber := rand.Float64()
	//运气不好没抢到
	if randomNumber > globalInfo.Probability {
		c.JSON(http.StatusOK, gin.H{
			"Code": SnatchFailed,
			"Msg":  "未抢到红包",
		})
		return
	}
	//得到红包金额
	revelopValue, err := generateEnvelopValue()
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusOK, gin.H{
			"Code": NoMoney,
			"Msg":  "没钱了",
		})
		return
	}
	envelopId, err := insertEnvelopes(request.Uid, revelopValue, curRevelopCount+1)
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusOK, gin.H{
			"Code": DatabaseWrong,
			"Msg":  "数据库错误",
		})
		return
	}
	c.JSON(http.StatusOK, SnatchRespond{
		Code: Succeed,
		Msg:  "success",
		Data: struct {
			EnvelopeId int64 `json:"envelopeId"`
			MaxCount   int   `json:"maxCount"`
			CurCount   int   `json:"curCount"`
		}{
			EnvelopeId: envelopId,
			MaxCount:   globalInfo.MaxReCount,
			CurCount:   curRevelopCount + 1,
		},
	})
	log.Println(request)
}

func openFunc(c *gin.Context) {
	request := OpenRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusOK, gin.H{
			"Code": JsonUnmarshalError,
			"Msg":  "解析错误",
		})
		return
	}
	//得到红包数量
	value, openedBefore, err := getEnvelopValue(request.EnvelopeId)
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusOK, gin.H{
			"Code": DatabaseWrong,
			"Msg":  "数据库错误",
		})
		return
	}
	if !openedBefore {
		updateUserValueSum(request.Uid, value)
	}
	c.JSON(http.StatusOK, OpenRespond{
		Code: Succeed,
		Msg:  "success",
		Data: struct {
			Value int `json:"value"`
		}{
			Value: value,
		},
	})
}

func getWalletListFunc(c *gin.Context) {
	request := GetWalletListRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusOK, gin.H{
			"Code": JsonUnmarshalError,
			"Msg":  "解析错误",
		})
		return
	}
	envelopes, err := getEnvelopes(request.Uid)
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusOK, gin.H{
			"Code": DatabaseWrong,
			"Msg":  "获取钱包失败",
		})
		return
	}
	respond := GetWalletListRespond{
		Code: Succeed,
		Msg:  "success",
	}
	respond.Data.Amount = 0
	for _, envelope := range envelopes {
		if envelope.Opened {
			respond.Data.Amount += envelope.Value
			respond.Data.EnvelopeList = append(respond.Data.EnvelopeList, gin.H{
				"envelope_id": envelope.EnvelopeId,
				"value":       envelope.Value,
				"opened":      true,
				"snatch_time": envelope.SnatchTime,
			})
		} else {
			respond.Data.EnvelopeList = append(respond.Data.EnvelopeList, gin.H{
				"envelope_id": envelope.EnvelopeId,
				"opened":      false,
				"snatch_time": envelope.SnatchTime,
			})
		}
	}
	c.JSON(http.StatusOK, respond)
}

// 获得一个红包金额，没钱会从总表中取出1/10的钱生成1/10总额的个红包
func generateEnvelopValue() (int, error) {
	info := GlobalInfo{}
	Db.First(&info)
	budget := info.Budget
	expenses := info.Expenses
	listKey := "envList"
	amount := getEnvAmount(listKey)
	if amount == 0 {
		// redis里没有预先生成好的红包了，向总表申请一笔钱重新生成
		money := budget / 10
		if expenses+money > budget {
			// 钱不够，无法再生成了
			return 0, errors.New("没钱了")
		}
		insertList(listKey, genEnvList(money, info.ALLEnvelopeNum/10))
		updateExpenses(money, info.ALLEnvelopeNum/10)
		amount = getEnvAmount(listKey)
	}
	return amount, nil
}

// 根据金额和数量生成待发红包列表
// 可以自由调整金额、数量，保证每个红包的期望是钱/人数的平均值
func genEnvList(money, num int64) []int {
	envList := make([]int, num)
	if money < num {
		panic("红包数太多，钱不够分了")
	}
	// 每个人先发1分钱
	for i := int64(0); i < num; i++ {
		envList[i] += 1
	}
	remainMoney := money - num
	remainNum := num
	// 剩余的钱随机发给每个人，每个人具体获得[0, 2*avg)范围内的一个随机整数
	rand.Seed(time.Now().UnixNano())
	for i := int64(0); i < num; i++ {
		avg := int(remainMoney / remainNum)
		randMoney := rand.Intn(2 * avg)
		// 如果钱不够发，则只发剩余钱数然后直接结束；如果只剩最后一人则全发
		if int64(randMoney) >= remainMoney || remainNum == 1 {
			envList[i] += int(remainMoney)
			remainMoney = 0
			remainNum = 0
			break
		}
		envList[i] += randMoney
		remainMoney -= int64(randMoney)
		remainNum--
	}
	return envList
}
