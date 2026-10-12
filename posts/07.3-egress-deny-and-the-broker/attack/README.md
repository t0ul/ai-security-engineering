# the attack

No input file; the attack is a connection request from inside the chamber. The VM
has no network card (last post), so when a tool genuinely needs the network it asks
a broker on the host to make the connection for it. A tool subverted by an email
asks the broker to reach an internal service instead of the web. The question is
whether the broker dials whatever it is asked, or only what is allowed.
