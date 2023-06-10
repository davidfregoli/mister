# Mister - Distributed MapReduce Framework

Repository for the Kubernetes runner and Library for MapReduce Mister Apps

---

**Mister is a study project and as such is not intended to be used in real-world scenarios (see: [Limitations](#limitations))**

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

https://github.com/davidfregoli/mister/assets/64096305/2bff65a0-6063-435e-b216-6db0fd669104

Example of a Indexer App run with 6 mapper and 3 reducer workers:

https://github.com/davidfregoli/mister/assets/64096305/a11a83ad-c72a-48d0-b9ec-52717214b7eb

[screen-capture (3).webm](https://github.com/davidfregoli/mister/assets/64096305/d506eb97-4e72-4073-8c07-d274043dbd63)
[screen-capture (4).webm](https://github.com/davidfregoli/mister/assets/64096305/27e5f516-d64e-42dd-817c-36f4d46d5d7a)


## Limitations
In order to manage the scope of the project, some architectural decisions are suboptimal or resemble antipatterns
- Inherently runs on a single node
- Most state is in-memory, some is derived from filesystem
- The k8s client is using the default service account and in-cluster config
- RPC endpoints are not secured
