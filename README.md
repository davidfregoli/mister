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


#### Example of a [Wordcount](https://github.com/davidfregoli/mister-wordcount) App run with 4 mapper and 2 reducer workers:

https://github.com/davidfregoli/mister/assets/64096305/ead41c05-52db-404e-bb41-68a0d6830f1d

#### Example of a [Indexer](https://github.com/davidfregoli/mister-indexer) App run with 5 mapper and 3 reducer workers:

https://github.com/davidfregoli/mister/assets/64096305/6eb3ee29-a0d1-481c-ae5a-19bef08cd22f


## Concepts explored in this study project
- exploring Go modules systems by:
  - setting up a vanity url
  - creating a Library module that exposes a simple API
  - creating Applications that import the Library module and elegantly take advantage of its Interfaces and API 
- using the `k8s.io/client-go` package to manage a cluster programmatically:
  - listing pods
  - spawning jobs
  - extracting logs
- Learning the basic Kubernetes terminology, components and tools, building a cluster and exposing a service from a YAML file
- writing Go templates with `html/template`
- using Go `http` to serve the Dashboard and the JSON API
- exploring different concurrency patterns in Go; throughout the history of the project sample implementations have used various facilities such as the `sync` primitives (locks, read locks, waitgroups), the `atomic` package, buffered and unbuffered channels

## Limitations
In order to manage the scope of the project, some architectural decisions are suboptimal or resemble antipatterns:
- Inherently runs on a single node
- Most state is in-memory, some is derived from filesystem
- The k8s client is using the default service account and in-cluster config
- RPC endpoints are not secured
- shell scripts expect `mister` and its apps to live under the `~/code` folder
- shell scripts always build and publish the container images to the local minikube registry with the fixed version tag `1`
- No way to specify or provide different input files directly from the dashboard, no way to seamlessly make a new Mister App available to the cluster

## Takeaways and Reflections
This project was conceived as a way to apply what I have learned by [solving](https://github.com/davidfregoli/mit65840/tree/main/mr) the MapReduce lab assignment from the MIT 6.5840 Distributed Systems class in a more real-world scenario, using industry-standard tools.

The initial plan turned out to be more of an excuse to make the first steps in the Kubernetes domain, which ultimately worked, but it was observed that some of the original problems didn't find a 1-to-1 mapping in the new ecosystem and were therefore set aside, partly or wholesale.

