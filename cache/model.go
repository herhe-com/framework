package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gookit/goutil/strutil"
	"github.com/herhe-com/framework/facades"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"reflect"
	"strings"
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

	if key != "" {
		facades.Redis.Del(tx.Statement.Context, keys(tx.Statement.Schema.Table, key))
	}
}

// FindById 优先从缓存中获取模型
func FindById(ctx context.Context, model any, id any) (err error) {

	t := reflect.TypeOf(model).Elem()

	if t.Kind() != reflect.Struct {
		return errors.New("model must be struct")
	}

	table := strutil.Snake(t.Name())

	result, err := facades.Redis.Get(ctx, keys(table, id)).Result()

	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	} else if err == nil {
		_ = json.Unmarshal([]byte(result), &model)
		return
	}

	tx := facades.Gorm.First(&model, id)

	if tx.Error == nil {
		hash, _ := json.Marshal(model)
		facades.Redis.Set(ctx, keys(table, id), string(hash), ttl())
	} else {
		return tx.Error
	}

	return nil
}

func id(tx *gorm.DB) string {

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
