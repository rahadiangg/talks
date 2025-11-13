package sharedmodule

type ThemeSelection string

const (
	ThemeTwentyTwentyFour ThemeSelection = "twentytwentyfour"
	ThemeTwentyTwentyTwo  ThemeSelection = "twentytwentytwo"
	ThemeTwentyTwentyFive ThemeSelection = "twentytwentyfive"
)

type HostingDetail struct {
	SubDomain string         `json:"subdomain"`
	Theme     ThemeSelection `json:"theme"`
	Email     string         `json:"email"`
}
