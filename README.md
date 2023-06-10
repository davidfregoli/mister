# Mister - Distributed MapReduce Framework

Repository for the Kubernetes runner and Library for MapReduce Mister Apps

---

**Mister is a study project and as such is not intended to be used in real-world scenarios (see:&nbsp;[Limitations](#limitations))**

---

This repository provides the module: `fregoli.dev/mister` which exports the package: `mister`

A Mister cluster depends on the 3 Go apps residing in `cmd/`:
- `fetch` creates the folder structure in the Persistent Volume Claim and downloads the text files of the books used as input  for the MapReduce Jobs.
- `coordinator` manages the state of the cluster, hosts the RPC endpoint, spins up the worker Pods and orchestates the MapReduce Jobs.
- `dashboard` provides the Web interface and the API endpoint.

The `mister` package is imported by `coordinator` and `dashboard`, as well as the standalone Mister Apps.

Writing a Mister App is straightforward:
- import `fregoli.dev/mister`
- create your app, a struct that implements the `mister.App` interface:
  ```
  type App interface {
      Map(filename string, contents string) []KeyValue
      Reduce(key string, values []string) string
  }
  ```
- call `mister.RegisterApp(yourApp)`


Example of a Wordcount App run with 4 mapper and 2 reducer workers:


https://github.com/davidfregoli/mister/assets/64096305/ead41c05-52db-404e-bb41-68a0d6830f1d

Example of a Indexer App run with 5 mapper and 3 reducer workers:

https://github.com/davidfregoli/mister/assets/64096305/6eb3ee29-a0d1-481c-ae5a-19bef08cd22f


## Limitations
In order to manage the scope of the project, some architectural decisions are suboptimal or resemble antipatterns
- Inherently runs on a single node
- Most state is in-memory, some is derived from filesystem
- The k8s client is using the default service account and in-cluster config
- RPC endpoints are not secured
