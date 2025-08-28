package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wantCase struct {
	Type             PostType
	Animal           AnimalType
	BreedContains    string
	Phones           []string
	ContactNamesHas  []string
	LocationContains []string
	WhenContains     []string
	AgeContains      string
	ExtrasSterilized bool
	ExtrasVaccinated bool
	ExtrasLitterOK   bool
}

func TestParse_Table(t *testing.T) {
	cases := []struct {
		name string
		text string
		want wantCase
	}{
		{
			name: "Lost cat with address and phone",
			text: "Пропала кошка! Срочно нужна помощь. Пушкинская улица, 283, Ижевск. Телефон: 79120281683. Имя хозяина: Юрий. Кошка Мася",
			want: wantCase{
				Type:             TypeLost,
				Animal:           AnimalCat,
				Phones:           []string{"+79120281683"},
				LocationContains: []string{"Пушкинская улица", "283"},
				ContactNamesHas:  []string{"Юрий"},
			},
		},
		{
			name: "Sighting dog no phone",
			text: "Бегает на Закирова и Первомайской кобелек, похож на Йорка, поймать не смогли",
			want: wantCase{
				Type:             TypeSighting,
				Animal:           AnimalDog,
				BreedContains:    "йорк",
				LocationContains: []string{"Закирова", "Первомайск"},
			},
		},
		{
			name: "Found cat with phone and name",
			text: "Найден кот. Район Заречное шоссе 49. 89127500184 Александр",
			want: wantCase{
				Type:             TypeFound,
				Animal:           AnimalCat,
				Phones:           []string{"+79127500184"},
				ContactNamesHas:  []string{"Александр"},
				LocationContains: []string{"Заречное шоссе", "49"},
			},
		},
		{
			name: "Adoption sterilized vaccinated",
			text: "Красивая кошечка, стерилизована, обработана. Лоток на отлично. 8-912-762-92-39 Ольга",
			want: wantCase{
				Type:             TypeAdoption,
				Animal:           AnimalCat,
				ExtrasSterilized: true,
				ExtrasLitterOK:   true,
				Phones:           []string{"+79127629239"},
				ContactNamesHas:  []string{"Ольга"},
			},
		},
		{
			name: "Fundraising",
			text: "Помочь оплатить передержку. Сумма к сбору. 📞 8912 4586329 Анна",
			want: wantCase{
				Type:            TypeFundraising,
				Phones:          []string{"+79124586329"},
				ContactNamesHas: []string{"Анна"},
			},
		},
		{
			name: "Lost with date time",
			text: "Потерялась кошка. Воткинское шоссе 39 26.08.2025 примерно в 22:00. 89120216801",
			want: wantCase{
				Type:             TypeLost,
				WhenContains:     []string{"26.08.2025"},
				LocationContains: []string{"Воткинское шоссе", "39"},
				Phones:           []string{"+79120216801"},
			},
		},
		{
			name: "Adoption kitten 2 months",
			text: "Малышу около 2х месяцев, лотком пользуется, пишите Юлии 89501684430",
			want: wantCase{
				Type:        TypeAdoption,
				AgeContains: "2",
				Phones:      []string{"+79501684430"},
			},
		},
		{
			name: "Link only",
			text: "https://vk.com/wall107929440_36",
			want: wantCase{Type: TypeLink},
		},
		{
			name: "Empty",
			text: "",
			want: wantCase{Type: TypeEmpty},
		},
		{
			name: "Lost multiple areas with name",
			text: "Мы потеряли кота. Районы Люкшудья и Шабердино. 8 922 405 26 12 - Татьяна",
			want: wantCase{
				Type:             TypeLost,
				Phones:           []string{"+79224052612"},
				ContactNamesHas:  []string{"Татьяна"},
				LocationContains: []string{"Люкшудья", "Шабердино"},
			},
		},
		{
			name: "Adoption extras",
			text: "стерилизована, вакцинирована. Лоток на отлично",
			want: wantCase{
				Type:             TypeAdoption,
				ExtrasSterilized: true,
				ExtrasVaccinated: true,
				ExtrasLitterOK:   true,
			},
		},
		{
			name: "Adoption duplicate content style",
			text: "Кошечка ищет дом, ласковая, 8 906 816 84 18",
			want: wantCase{
				Type:   TypeAdoption,
				Animal: AnimalCat,
				Phones: []string{"+79068168418"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(0, tc.text)

			require.Equal(t, tc.want.Type, got.Type, "Type")

			if tc.want.Animal != "" {
				assert.Equal(t, tc.want.Animal, got.Animal, "Animal")
			}
			if tc.want.BreedContains != "" {
				assert.Contains(t, strings.ToLower(got.Breed), tc.want.BreedContains, "BreedContains")
			}
			if tc.want.AgeContains != "" {
				assert.Contains(t, got.Age, tc.want.AgeContains, "AgeContains")
			}
			if tc.want.Phones != nil {
				assert.Equal(t, tc.want.Phones, got.Phones, "Phones")
			}
			if len(tc.want.LocationContains) > 0 {
				assert.Truef(t, containsAll(got.Location, tc.want.LocationContains), "Location: %q missing %v", got.Location, tc.want.LocationContains)
			}
			if len(tc.want.WhenContains) > 0 {
				assert.Truef(t, containsAll(got.When, tc.want.WhenContains), "When: %q missing %v", got.When, tc.want.WhenContains)
			}
			if len(tc.want.ContactNamesHas) > 0 {
				assert.Truef(t, sliceHasAll(got.ContactNames, tc.want.ContactNamesHas), "ContactNames: %v missing %v", got.ContactNames, tc.want.ContactNamesHas)
			}
			if tc.want.ExtrasSterilized {
				assert.True(t, got.Extras.Sterilized, "Extras.Sterilized")
			}
			if tc.want.ExtrasVaccinated {
				assert.True(t, got.Extras.Vaccinated, "Extras.Vaccinated")
			}
			if tc.want.ExtrasLitterOK {
				assert.True(t, got.Extras.LitterOK, "Extras.LitterOK")
			}
		})
	}
}

// --- helpers ---

func containsAll(hay string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(hay, n) {
			return false
		}
	}
	return true
}

func sliceHasAll(hay []string, needles []string) bool {
	set := map[string]bool{}
	for _, v := range hay {
		set[v] = true
	}
	for _, n := range needles {
		if !set[n] {
			return false
		}
	}
	return true
}

func sliceHasAny(hay []string, needles []string) bool {
	set := map[string]bool{}
	for _, v := range hay {
		set[v] = true
	}
	for _, n := range needles {
		if set[n] {
			return true
		}
	}
	return false
}

// Using testify for equality checks in tests; helpers above cover contains cases.
