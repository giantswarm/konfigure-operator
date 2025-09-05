## How to run locally?

### Create test cluster

First, create a `kind` cluster and bootstrap core app platform for it.

```shell
kind create cluster --name kind
apptestctl bootstrap --kubeconfig="$(kind get kubeconfig --name kind)"
```

Used `apptestcl` version `0.22.1`: https://github.com/giantswarm/apptestctl/releases/tag/v0.22.1.

### Build image and install Helm chart

You want to rerun this script if you are making changes to the operator code or the Helm chart to test against the
latest state.

From the root of the repository:

```shell
./tests/ats/setup-local.sh
```
### Setup python to run the tests

```shell
cd tests/ats

# Make sure the python version here matches the one in tests/ats/Pipfile under [requires]/python_version.
# Also make sure you have the same version in the ATS test container for the ATS version configured in .circleci/config.yml.
pipenv --python 3.12
pipenv update
pipenv shell

# Then you must point KUBECONFIG to the kind cluster! Its important, be careful not to run against a real cluster.
kind get kubeconfig --name kind > /tmp/kind.config
export KUBECONFIG=/tmp/kind.config

# Finally, run the test with:
pytest -v
```

The initial run will be slower, cos it installs Flux and waits for it to be ready.

Consecutive local runs will update the test resources to make it faster. On CI, you always run against an empty cluster.
