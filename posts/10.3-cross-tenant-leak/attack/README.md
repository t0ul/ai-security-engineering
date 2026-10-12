# the attack

No input file; the attack is an ordinary question asked against a shared index.
The store holds two households' emails. Alice's private line, a therapy
appointment, lives under her tenant; Bob's PTA note lives under his; a school
notice is public, under no tenant.

rag-tenant-leak is Bob asking "what appointments do I have?" and getting Alice's.
It does not require a malicious query. It only requires a retrieval that searches
the whole index and forgets to scope results to the asker. The leaked line is a
benign marker standing in for whatever private email a real tenant holds. The
question is whether authorization is part of the search itself, or an afterthought
the retrieval path skipped.
