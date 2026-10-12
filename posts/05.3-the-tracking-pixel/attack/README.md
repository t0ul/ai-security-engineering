# the attack

No input file; the attack is the model's output. A hostile email got the model to
put a tracking pixel into the summary it shows the user: a markdown image whose URL
carries a session token in its query string. The user never clicks anything; a
browser fetches an image as soon as the page loads, so the token leaves the instant
the summary is displayed. `maliciousOutput` in main.go is that response.
