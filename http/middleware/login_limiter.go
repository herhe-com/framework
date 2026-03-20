package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/contracts/http/response"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/http"
	"github.com/herhe-com/framework/support/util"
)

// LoginLimiter 登录失败限制中间件，用于防止暴力破解
// 所有配置从 cfg 中读取：
// - auth.login.max_attempts: 最大失败次数，默认 5
// - auth.login.lock_duration: 锁定时长（分钟），默认 15
// - auth.login.show_attempts: 是否显示失败次数提示，默认 false
// - auth.login.identifier_field: 用户标识符字段名（如 username、email 等），默认 username
// - auth.login.lock_message: 账户锁定提示消息，默认 "Account is locked. Please try again in %d minutes."
// - auth.login.attempts_message: 失败次数提示消息，默认 "%s (Failed %d times, %d attempts remaining before account lock)"
func LoginLimiter() app.HandlerFunc {

	// Lua 脚本：检查锁定状态
	checkLockScript := `
		local lockKey = KEYS[1]
		local locked = redis.call("EXISTS", lockKey)
		if locked == 1 then
			local ttl = redis.call("TTL", lockKey)
			return {1, ttl}
		end
		return {0, 0}
	`

	// Lua 脚本：处理登录失败，原子性地增加失败次数并检查是否需要锁定
	handleFailureScript := `
		local attemptsKey = KEYS[1]
		local lockKey = KEYS[2]
		local maxAttempts = tonumber(ARGV[1])
		local lockDuration = tonumber(ARGV[2])
		
		-- 增加失败次数
		local attempts = redis.call("INCR", attemptsKey)
		
		-- 如果是第一次失败，设置过期时间
		if attempts == 1 then
			redis.call("EXPIRE", attemptsKey, lockDuration)
		end
		
		-- 检查是否达到最大失败次数
		if attempts >= maxAttempts then
			-- 锁定账户
			redis.call("SET", lockKey, "1", "EX", lockDuration)
			-- 清除失败次数记录
			redis.call("DEL", attemptsKey)
			return {attempts, 0}
		end
		
		-- 返回当前失败次数和剩余次数
		return {attempts, maxAttempts - attempts}
	`

	return func(c context.Context, ctx *app.RequestContext) {

		// 从配置文件读取所有配置
		maxAttempts := facades.Cfg.GetInt64("auth.login.max_attempts", 5)
		lockMinutes := facades.Cfg.GetInt("auth.login.lock_duration", 15)
		lockDuration := time.Duration(lockMinutes) * time.Minute
		showAttempts := facades.Cfg.GetBool("auth.login.show_attempts", false)
		identifierField := facades.Cfg.GetString("auth.login.identifier_field", "username")
		lockMessage := facades.Cfg.GetString("auth.login.lock_message", "Account is locked. Please try again in %d minutes.")
		attemptsMessage := facades.Cfg.GetString("auth.login.attempts_message", "%s (Failed %d times, %d attempts remaining before account lock)")

		// 绑定请求数据到 map
		var requestData map[string]any
		if err := ctx.Bind(&requestData); err != nil {
			ctx.Next(c)
			return
		}

		// 从 map 中获取用户标识符
		identifier := ""
		if val, exists := requestData[identifierField]; exists {
			identifier = fmt.Sprintf("%v", val)
		}
		if identifier == "" {
			ctx.Next(c)
			return
		}

		// 检查 Redis 是否可用
		if facades.Redis == nil {
			ctx.Next(c)
			return
		}

		// 生成 Redis key
		lockKey := util.Keys("login:lock", identifier)
		attemptsKey := util.Keys("login:attempts", identifier)

		// 使用 Lua 脚本检查是否被锁定（单次 Redis 调用）
		result, err := facades.Redis.Default().Eval(c, checkLockScript, []string{lockKey}).Result()
		if err == nil {
			if resultSlice, ok := result.([]interface{}); ok && len(resultSlice) == 2 {
				if locked, ok := resultSlice[0].(int64); ok && locked == 1 {
					if ttl, ok := resultSlice[1].(int64); ok {
						ctx.Abort()
						http.Fail(ctx, lockMessage, ttl/60+1)
						return
					}
				}
			}
		}

		// 继续处理请求
		ctx.Next(c)

		// 解析响应体判断登录是否成功
		var resp response.Response[any]
		if err := json.Unmarshal(ctx.Response.Body(), &resp); err == nil {
			// 通过 Code 字段判断登录是否成功（Code == 20000 表示成功）
			if resp.Code == 20000 {
				// 登录成功，清除失败记录
				facades.Redis.Default().Del(c, attemptsKey)
				return
			}
		}

		// 登录失败，使用 Lua 脚本原子性地处理失败逻辑（单次 Redis 调用）
		result, err = facades.Redis.Default().Eval(c, handleFailureScript,
			[]string{attemptsKey, lockKey},
			maxAttempts, int(lockDuration.Seconds())).Result()

		if err != nil {
			return
		}

		// 解析结果并处理失败次数提示
		if resultSlice, ok := result.([]interface{}); ok && len(resultSlice) == 2 {
			if attempts, ok := resultSlice[0].(int64); ok {
				if remaining, ok := resultSlice[1].(int64); ok && remaining > 0 && showAttempts {
					// 如果开启了失败次数提示，修改响应消息
					resp.Message = fmt.Sprintf(attemptsMessage, resp.Message, attempts, remaining)

					// 重新序列化响应体
					if body, err := json.Marshal(resp); err == nil {
						ctx.Response.SetBody(body)
					}
				}
			}
		}
	}
}
