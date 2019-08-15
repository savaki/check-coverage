check-coverage
------------------------------

`check-coverage` uses code coverage as a metric and requires that if code coverage is below X, 
builds may not decrease code coverage.


#### Usage

Ensure that builds do not lower code coverage until a minimum coverage level of 80% is met.  In
this example, we're submitting build results of 40% coverage for a branch name, `develop`.

If 40% is lower then the previous coverage level, this build breaks.

```bash
check-coverage \
  --coverage 40 \
  --minimum 80 \
  --repository bitbucket.org/example/project \
  --branch develop \
  --hash b4570ff4af18a2e0978e6dff69dfd56c9eb44070 \
  --table coverage
```