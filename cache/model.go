package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/support/util"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type Model struct {
}

func (m *Model) AfterUpdate(tx *gorm.DB) (err error) {
	m.clear(tx)
	return
}

func (m *Model) AfterDelete(tx *gorm.DB) (err error) {
	m.clear(tx)
	return
}

// 数据修改之后，自动删除缓存模型
func (m *Model) clear(tx *gorm.DB) {

	key := id(tx)

	if cache, ok := facades.OptionalRedis(); lo.IsNotEmpty(key) && ok {
		cache.Default().Del(tx.Statement.Context, util.Keys(tx.Statement.Schema.Table, key))
	}
}

// FindByID 优先从缓存中获取模型
func FindByID(ctx context.Context, model any, id any) (err error) {

	t := reflect.TypeOf(model).Elem()

	if t.Kind() != reflect.Struct {
		return errors.New("model must be struct")
	}

	// 检查是否有多个 primary key
	if err := checkPrimaryKeys(model); err != nil {
		return err
	}

	table := lo.SnakeCase(t.Name())

	v := reflect.ValueOf(model)

	method := v.MethodByName("TableName")

	if method.IsValid() {
		values := method.Call(nil)
		if len(values) == 1 {
			table = values[0].String()
		}
	}

	key := util.Keys(table, id)

	result, err := facades.Redis().Default().Get(ctx, key).Result()

	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	} else if err == nil {
		_ = json.Unmarshal([]byte(result), &model)
		return
	}

	tx := facades.Database().Default().First(&model, id)

	if tx.Error == nil {
		hash, _ := json.Marshal(model)
		facades.Redis().Default().Set(ctx, key, string(hash), TTL())
	} else {
		return tx.Error
	}

	return nil
}

func id(tx *gorm.DB) string {

	// 检查是否有多个 primary key
	primaryKeyCount := 0
	for _, field := range tx.Statement.Schema.Fields {
		if field.PrimaryKey {
			primaryKeyCount++
		}
	}

	if primaryKeyCount > 1 {
		// 记录错误但不中断流程，返回空字符串表示无法处理
		tx.AddError(fmt.Errorf("cache model does not support composite primary keys, found %d primary keys", primaryKeyCount))
		return ""
	}

	var ids = make([]string, 0)

	for _, field := range tx.Statement.Schema.Fields {
		if field.Name == tx.Statement.Schema.PrioritizedPrimaryField.Name {
			switch tx.Statement.ReflectValue.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < tx.Statement.ReflectValue.Len(); i++ {
					// 从字段中获取数值
					if fieldValue, isZero := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue.Index(i)); !isZero {
						ids = append(ids, fmt.Sprintf("%v", fieldValue))
					}
				}
			case reflect.Struct:
				// 从字段中获取数值
				if fieldValue, isZero := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue); !isZero {
					ids = append(ids, fmt.Sprintf("%v", fieldValue))
				}
			}
		}
	}

	return strings.Join(ids, "-")
}

// checkPrimaryKeys 检查模型是否有多个 primary key
func checkPrimaryKeys(model any) error {
	// 使用临时 DB 实例解析 schema
	db := facades.Database().Default()
	if db == nil {
		return errors.New("database not initialized")
	}

	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return fmt.Errorf("failed to parse model schema: %w", err)
	}

	// 统计 primary key 数量
	primaryKeyCount := 0
	for _, field := range stmt.Schema.Fields {
		if field.PrimaryKey {
			primaryKeyCount++
		}
	}

	if primaryKeyCount > 1 {
		return fmt.Errorf("cache model does not support composite primary keys, found %d primary keys", primaryKeyCount)
	}

	return nil
}
