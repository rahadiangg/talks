module fg-notification

go 1.23.0

require (
	huaweicloud.com/go-runtime v0.0.0-00010101000000-000000000000
	sharedmodule v0.0.0
)

require github.com/joho/godotenv v1.5.1

replace huaweicloud.com/go-runtime => ../go-runtime

replace sharedmodule => ../sharedmodule
