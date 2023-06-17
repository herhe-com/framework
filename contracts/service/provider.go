package service

type Provider interface {
	//Boot any application services after register.
	Boot() error
	//Register any application services.
	Register() error
}
