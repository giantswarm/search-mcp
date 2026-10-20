package search

import (
	"net/http"

	"github.com/giantswarm/pagekit"
)

// productName names search-mcp on its pages.
const productName = "Giant Swarm search"

// signedInPage is shown after a sign-in from an AI assistant or the browser.
var signedInPage = pagekit.Page{
	Title:   "Signed in to " + productName,
	Heading: "Signed in",
	Summary: productName + " is now connected",
	Message: "You can close this tab and return to where you started the sign-in.",
}

// deviceSignedInPage is shown after a device flow sign-in; userCode is the
// code the person entered.
func deviceSignedInPage(userCode string) pagekit.Page {
	p := signedInPage
	p.Summary = "Your device is now connected"
	p.Message = "You can close this tab and return to your device."
	p.Detail = userCode
	return p
}

// renderSignInError writes a sign-in error page. summary names the cause and
// message what to do next.
func renderSignInError(w http.ResponseWriter, status int, summary, message string) {
	pagekit.Render(w, status, pagekit.Page{
		Title:   "Sign-in to " + productName + " failed",
		Heading: "Sign-in failed",
		Summary: summary,
		Message: message,
	})
}

// Next steps of the sign-in error pages.
const (
	messageStartAgain = "You can close this tab and start the sign-in again from where you started it."
	messageTryLater   = "Try again in a moment."
	messageEnterCode  = "Go back and enter the code shown on your device again."
	// messageContactSupport is for a failure retrying cannot fix.
	messageContactSupport = "This is a problem on our side. Contact Giant Swarm support."
)
