# SPEC 07.2-the-detonation-chamber

- part: P4 ch7 isolation
- title: The detonation chamber
- spec: prose only (the chamber is a codesigned Apple VM, not an importable module; demo is a LIVE VM smoke from the emaildrop checkout) -> Apple VM, vsock only, no NIC -> code runs with no path to host -> live VM smoke

Kind: PROSE-ONLY (no portable lesson module; the real launchvm detonation chamber
is demonstrated live from ../emaildrop, the "after" to 07.1).
Done when: index.md passes make lint, and the VM-smoke receipt is a REAL live run
against the booted chamber (no host fs, no NIC, no internet, host untouched).
