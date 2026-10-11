// Command main is the runnable demo for one lesson. Copy this folder, then wire
// the two paths below to the real attack and the real control for this post.
//
// Run it with:
//
//	go run .
//
// It must do two things: run the attack against the undefended code and show it
// landing, then run the same attack against the fixed code and show it blocked.
// The undefended path genuinely omits the control. No stubbed results.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("== before: no defense ==")
	if landed := attackUndefended(); landed {
		fmt.Println("attack landed (this is the problem the post fixes)")
	} else {
		fmt.Println("TODO: the undefended path should let the attack land")
		os.Exit(1)
	}

	fmt.Println("\n== after: with the control ==")
	if blocked := attackDefended(); blocked {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("TODO: the defended path should block the attack")
		os.Exit(1)
	}
}

// attackUndefended runs this post's attack against the code with NO control in
// place. It returns true when the attack succeeds (which, before the fix, it
// should). Replace the body with the real, minimal undefended version.
func attackUndefended() bool {
	// TODO: build the naive feature (no control) and run attack/ against it.
	return true
}

// attackDefended runs the SAME attack against the code once the control from this
// post is added. It returns true when the attack is blocked. Replace the body with
// the real control, importing its library.
func attackDefended() bool {
	// TODO: add the real control (import the library) and run the same attack.
	return true
}
