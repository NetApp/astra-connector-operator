This is a small pytest suite POC. This was created show what CRD tests in pytest could look like and to help make
a decision on what test framework we want to use for CRD testing.

This is currently living in the connector-operator repo but would be moved to the Neptune repo if we choose to go
this route.

There are still todos and design that could be improved, but for POC purposes it's good enough.

See python_tests/management_tests/test_neptune.py for the tests.

* Expects neptune controllers to already be installed on whatever you're using to test with
* Point to your test cluster via --kubeconfig param
* Has automatic fixture based cleanup (minimal, just enough for POC)
* see conftest.py file for input args and fixture defs

