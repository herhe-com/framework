package request

import contractcaptcha "github.com/herhe-com/framework/contracts/captcha"

// Captcha contains the user input required to verify a captcha.
type Captcha = contractcaptcha.VerifyData
