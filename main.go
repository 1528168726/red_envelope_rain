package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"math/rand"
	"net/http"
)

var globalInfo GlobalInfo

func main() {

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
	revelopValue := generateRevelopValue()
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

//红包金额算法：未作优化
func generateRevelopValue() int {
	return 1
}
