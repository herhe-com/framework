package pms

import "github.com/golang-module/carbon/v2"

type Driver interface {
	Search()
	Login(mobile, code string) //	验证码登陆
	Register()
	SmsOfLogin(mobile string) error //	发送验证码：	注册 / 登陆 / 改密
	Moneys(hotel string, types, cards []string, begin, end carbon.DateTime) ([]any, error)
	Stocks(hotel string, types []string, begin, end carbon.DateTime) ([]StocksResponse, error)
	Booking()
	Cancel()
	Information()
	Status()
	Coupons()
	Pay()
	Refund()
}
