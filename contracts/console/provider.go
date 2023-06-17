package console

type Provider interface {
	//Register any application services.
	Register() Console
}
