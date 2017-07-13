package dialog

type Variant struct {
	Id   string
	Text string
}

type Dialog struct {
	Id       string
	Text     string
	Variants []Variant
}
