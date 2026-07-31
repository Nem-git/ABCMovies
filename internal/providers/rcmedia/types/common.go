package types

type Badge struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type Image struct {
	URL     string `json:"url"`
	AltText string `json:"altText"`
	Size    string `json:"size"`
}

type Images struct {
	Background *Image `json:"background"`
	Card       *Image `json:"card"`
	Logo       *Image `json:"logo"`
	Network    *Image `json:"network"`
}

type Type struct {
	AvailabilityType string `json:"availabilityType"`
	Tiers            string `json:"tiers"`
}

type AppleMediaServiceSubscriptionV2 struct {
	Expires int   `json:"expires"`
	Type    *Type `json:"type"`
}

type HTMLMeta struct {
	Title                           string                           `json:"title"`
	Description                     string                           `json:"description"`
	OGTitle                         string                           `json:"og:title"`
	OGDescription                   string                           `json:"og:description"`
	OGURL                           string                           `json:"og:url"`
	OGType                          string                           `json:"og:type"`
	OGImage                         string                           `json:"og:image"`
	OGImageWidth                    int                              `json:"og:image.width"`
	OGImageHeight                   int                              `json:"og:image.height"`
	AppleMediaServiceSubscriptionV2 *AppleMediaServiceSubscriptionV2 `json:"apple-media-service-subscription-v2"`
	AppleItunesApp                  string                           `json:"apple-itunes-app"`
	ALIOSURL                        string                           `json:"al:ios:url"`
	ALIOSAppStoreID                 string                           `json:"al:ios:app_store_id"`
	ALIOSAppName                    string                           `json:"al:ios:app_name"`
	ALAndroidURL                    string                           `json:"al:android:url"`
	ALAndroidAppName                string                           `json:"al:android:app_name"`
	ALAndroidPackage                string                           `json:"al:android:package"`
	ALWebURL                        string                           `json:"al:web:url"`
}
