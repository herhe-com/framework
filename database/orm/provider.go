package orm

import (
	"github.com/herhe-com/framework/contracts/database"
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (p *ServiceProvider) Register() (err error) {

	db, err := NewApplication()

	if err != nil {
		return err
	}

	facades.Register[database.DB](db)

	return nil
}

func (p *ServiceProvider) Boot() error {
	return nil
}
