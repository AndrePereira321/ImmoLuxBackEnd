package utils

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Lisboa", "lisboa"},
		{"Porto", "porto"},
		{"Viana do Castelo", "viana-do-castelo"},
		{"São Brás de Alportel", "sao-bras-de-alportel"},
		{"Évora", "evora"},
		{"Setúbal", "setubal"},
		{"Castelo Branco", "castelo-branco"},
		{"Coimbra", "coimbra"},
		{"Figueira da Foz", "figueira-da-foz"},
		{"", ""},
		{"porto", "porto"},
		{" Lisboa ", "lisboa"},
		{"Vila  Real", "vila-real"},
	}
	for _, c := range cases {
		result := Slugify(c.input)
		if result != c.expected {
			t.Errorf("Slugify(%q) = %q; want %q", c.input, result, c.expected)
		}
	}
}
