# the attack

No input file; the target is the kill switch itself, under two pressures.

unauthz-killswitch: whoever can toggle the switch controls the incident. On an
ungoverned switch, an attacker who reaches it flips it back off mid-response, or
engages a full halt as a denial of service. Toggling it must require a break-glass
operator capability, so an app-scoped role cannot touch it.

deny-storm: a single all-or-nothing switch makes its own outage. A full halt in
response to one misbehaving tool refuses every legitimate read too, a self-inflicted
denial of service. A layered switch stops side effects while reads keep serving, and
escalates to a full halt only when that is actually warranted.
