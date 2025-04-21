# kind-examples

A good ol' friend of mine gave me a kind bootstrap script for me to start to get
more close to how k8s works. So I decided to hop on an adventure to really
figure it out. I already had some high-level understanding of how it works and
what is actually used to. It's interesting to mention that I also maintain a
couple of servers on my spare time and have long been an advocate of
self-hosting, but my tooling has mostly been ansible or just raw-dogging
commands/scripts.

But Kubernetes always felt like “take something basic and increase its
complexity into oblivion.” Which is obviously fine for, say, a large company
that needs serious scalability. But how does that complexity serve a small user
or a tiny team?

## Let's be kind with each other

So, `kind`, doesn't exactly _solve_ this problem or answer our question. But it
provides us with the ability to leverage our knowledge of docker to enter the
world of k8s, by making it possible to run k8s nodes as Docker containers
instead of spinning it up VMs or rely on the Mighty Cloud. Installing it was a
breezy:

`apk add kind kubeclt kubeadm # you'll need all of them`

So, the general gist is: `kind` lets you define a Kubernetes cluster using a
YAML config file and spint it up as local containers. Here's the original
configuration file my friend gave me:

```yml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: local
nodes:
  - role: control-plane
  - role: worker
    labels:
      app: backend

  - role: worker
    labels:
      app: frontend

  - role: worker
    labels:
      app: backend

  - role: worker
    labels:
      app: frontend

  - role: worker
    labels:
      app: db

  - role: worker
    labels:
      app: db
```

This creates a Kubernetes cluster with one control plane node and six worker
nodes - grouped by labels like `backend`, `frontend` and `db`. It's a bit closer
to how you'd structure a real-world application which is hella cool because it
gives you a more realistic environment to play with than say a single node where
you'll run a blog or static website.

We'll expand on this configuration and our main focus here will be deploying the
backend and frontend apps, without relying on the database for now.

## The applications

In the `src/` folder, I've created two apps. each with its own `Dockerfile`.

- Frontend: A barebones, PicoCSS-inspired frontend that fetches data from the
  backend and renders it in a table. Pico is great for these kinds of quick
  setups - it's classless and looks decent without any extra effort.
- Backend: A tiny Go app using `go-chi`. It exposes three endpoints: a health
  check, a basic hello world, and an API that lists a few stars with their size
  and distance.

I went with Go because:

- Node.js always drags in a mountain of dependencies and config
- Python also needs extra setup, requirements.txt, pip woes, env, etc
- Go just builds and runs. Fast, easy. Done.


## Containers

With our apps ready, the next step is to build Docker images, build the cluster
and deploy it there.

Let's put the cluster up first. We'll use the `kind` command to do that.

`kind create cluster --config kind-config.yml`

This will create a cluster for us, name it "local", via our configuration file.

We should also make `kubectl` be aware to run all latter commands under this
context:

`kubeclt config use-context kind-local`

And now, the magic: we need to build the images, based on our container
definitions to run these applications. If you actually have a running container
you want to use you can just skip this entirely. We'll stick with the basics for
now.

`docker build -t kind-backend-sample src/backend`

You'll want to do the same for the frontend and tag it `kind-frontend-sample`.
By the way, it's a nice opportunity to test it out as well:

`docker run -p 3000:3000 -it kind-backend-sample`, which will run the latest
image with the tag `kind-backend-sample`. The `p` flag allow us to publish the
ports we want from the host to the container. Make sure you can run both images
here, so we have absolute sure our containers works properly as it is.

### Loading the images

Since `kind` runs a local cluster in Docker, it won't pull your images from
Docker Hub, so we need to (1) load the images into the cluster and (2) tell the
cluster to use the local images instead of pulling them from a registry.

```bash
kind load docker-image kind-backend-sample --name local
kind load docker-image kind-frontend-sample --name local
```

You can see we'll patch our deployments to use the local images instead of the
ones in a registry with these lines:

```yml
      containers:
      - name: backend
        image: kind-backend-sample
        imagePullPolicy: Never
```

This will tell k8s to not look up any registry for the image, and use the local
one instead. This is important, because if you don't do this, k8s will try to
pull the image from a registry, and it won't find it, and it'll fail. You can
also use `IfNotPresent` instead of `Never`, but that's not the point here.

## Putting these big bois up

If you’re familiar with Docker Compose, Kubernetes has a similar mental model,
but with extra stuff.

- Images: same idea
- Pods: one or more containers grouped together (usually 1 per Pod in simple
  setups)
- Deployments: describe how to spin up Pods, how many replicas, restart
  policies, etc.
- Services: expose your Pods within the cluster or to the outside world
- Ingress: route traffic to your services (like a reverse proxy)
- ConfigMaps: store configuration data
- Secrets: store sensitive data (like passwords, tokens, etc)
- Volumes: persistent storage for your Pods
- Namespaces: separate resources within the cluster

With this in mind, we'll create a deployment for each of our apps, and expose it
via the ingress controller. The latter will have its own namespace (and a bunch
of extra stuff as well).

Let's begin with our side:

```bash
[1:09:34] butia ~/s/kind-examples
% kubectl apply -f backend-deployment.yml

[1:09:46] butia ~/s/kind-examples
% kubectl apply -f frontend-deployment.yml
```

### Breaking it down

This file defines two things. A deployment and a service.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      nodeSelector:
        app: backend
      containers:
      - name: backend
        image: kind-backend-sample
        imagePullPolicy: Never
        ports:
        - containerPort: 3000

```

It defines a couple of important things here. First, some metadata, which is
used to identify the deployment. Then, we have the spec, which is where we
actually define what do we want to do for this deployment. So, we have the
amount of replicas `spec.replicas` set to 2. This means we want to run two
instances of this deployment.

Then we have the selector, which is used to identify the pods that belong to
this deployment. So k8s knows which workers to use to run the pods. This will
combine with the info we defined in the cluster config.

Then, we have the template, which is used to define the pod that will be created
for each replica. Breaking it further down:

- `template.metadata.labels` Labels applied to each Pod. Must match the
  spec.selector above
- `template.spec.nodeSelector` Restricts scheduling: only nodes labeled
  app=backend will accept these Pods. Ideal for multi-role clusters, which is
  our case
- `template.spec.containers` The list of containers to run in each Pod:
  - `name: backend` Logical name for this container within the Pod
  - `image: kind-backend-sample` Uses the locally-loaded Docker image
  - `imagePullPolicy: Never` Skips registry pulls, relying on the loaded local image
  - `ports.containerPort: 3000` Exposes port 3000 inside the container (where
    the Go server listens)

So recaping the deployment: it creates two Pods, allocating to the correct
nodes/workers. Monitors them, and restarts them if they fail, all while ensuring
that the pods are always spread (given that we have the resource availability
and subject to the rules we defined, like the nodeSelector).

Let's check the service now:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: backend
spec:
  selector:
    app: backend
  ports:
    - protocol: TCP
      port: 80
      targetPort: 3000
  type: ClusterIP
```

The kind there defined that this object is a service, which routes network
traffic to Pods. It applies some metadata to identify the service, and then we
have the spec we want to use it.

`spec.selector` is used to identify the Pods that will receive the traffic.
It'll look for Pods with the label `app=backend`, which is the same label we
used in the Deployment.

`spec.ports` is used to define the ports that will be used to route the the
traffic. It's a bit confusing at first, but the gist is:
- `port` the port the Service exposes **inside the cluster**
- `targetPort` the port on the Pod/container that the traffic will be forwared
   to
- `protocol` the protocol used to route the traffic (TCP in our case)
- `type` the type of service. In our case, we are using `ClusterIP`, which
  means that this service will only be accessible from inside the cluster. This
  is the default type, and it's the one we want to use for our backend and
  frontend. So Frontend Pods and the Ingress can use `backend:80` for example to
  reach it. You probably have seen this in a `docker-compose` file before.

So recaping the service: it gets a internal IP address, reachable from the
Cluster. It **load-balances** incoming requests accross all healthy Pods
matching that selector. Pods can be added or removed (scaled up or down) without
changing how clients reach the service.

#### How they fit together

1. Deployment spins up multiple Pods of your applications
2. Each Pod registers itself with the Service, via its labels or other selectors
3. The Service provides a single DNS name for them (backend/frontend/db) and
   their respective IP
4. Any client inside the cluster (frontend Pods or Ingress) sends traffic to
   their DNS name (`http://backend:80` or whatever port is defined in `port`)
5. The Service load-balances to container ports on all available instances of
   that given application (backend in this example)

It's just separation of concerns. **Deployment** for lifecycle management &
scaling. **Service** for networking and load-balancing (update, scaling,
rollback the application without touching how other components connect to it).

They are often paired, but you don't always need them together in every
scenario. For example k8s allows you to have `DaemonSets` or `Jobs`, which are
temporary and don't usually need a service. You can also have Cluster-internal
only Pods that will only talk to each other (though Services allows better
Discoverability).

You'll also notice that they are usually defined in the same file, mostly for
better organization to apply it all at once.

### Backing it up

After all that jazz, we can check if they are up:

```bash
 [1:21:34] butia ~/s/kind-examples
% kubectl get deployments
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
backend    1/1     1            1           49m
frontend   1/1     1            1           45m
```

Which tells you that you have two deployments, each with their respective
instance/replicas running up to speed, and two services as well (one being the
control plane):

```bash
 [1:22:33] butia ~/s/kind-examples
% kubectl get svc
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
backend      ClusterIP   10.96.51.50     <none>        80/TCP    124m
frontend     ClusterIP   10.96.117.183   <none>        80/TCP    43m
kubernetes   ClusterIP   10.96.0.1       <none>        443/TCP   4h26m
```

### A Control Plane?

Your control plane is basically the “brain” of your Kubernetes cluster, so it’s
the set of components that decide what gets scheduled where, keep track of
desired state, and expose the API you talk to with `kubectl`. In a real cluster
you’d typically have more, but with `kind` it’s just one Docker container
running all these services:

- `kube-apiserver`: the frontdoor for all `kubectl` or client requests. It
    persists all the state of the cluster in a database (etcd)
- `etcd` (I love that name). A lightweight, distributed key-value store. It has
    all your cluster configuration data. Similar to how `/etc` holds your
    system configuration
- `kube-controller-manager`: runs a bunch of controllers for deployment, node,
    so it keeps it up with the `etcd` state and takes corrective actions
- `kube-scheduler`: watches for new Pods that have no assigned node and decide
    which fellas to assign them to, based on a myriad of factors (selectors,
    affinity, taints, etc)

You can check the status of the control plane by running:

```bash
% kubectl get pods -n kube-system
NAME                                          READY   STATUS    RESTARTS   AGE
coredns-668d6bf9bc-cdcdw                      1/1     Running   0          74m
coredns-668d6bf9bc-j4lzp                      1/1     Running   0          74m
etcd-local-control-plane                      1/1     Running   0          75m
kindnet-gsn7g                                 1/1     Running   0          74m
kindnet-hfchr                                 1/1     Running   0          74m
kindnet-m9pcd                                 1/1     Running   0          74m
kindnet-p4f9f                                 1/1     Running   0          74m
kindnet-p68nz                                 1/1     Running   0          74m
kindnet-rx4fl                                 1/1     Running   0          74m
kindnet-t9qk8                                 1/1     Running   0          74m
kube-apiserver-local-control-plane            1/1     Running   0          75m
kube-controller-manager-local-control-plane   1/1     Running   0          75m
kube-proxy-229wm                              1/1     Running   0          74m
kube-proxy-24z8f                              1/1     Running   0          74m
kube-proxy-578ff                              1/1     Running   0          74m
kube-proxy-5x9xb                              1/1     Running   0          74m
kube-proxy-66mhb                              1/1     Running   0          74m
kube-proxy-7nc4w                              1/1     Running   0          74m
kube-proxy-tlds6                              1/1     Running   0          74m
kube-scheduler-local-control-plane            1/1     Running   0          75m

```

### Exposing the services

You can do a quick test of the services by running:

```bash
% kubectl port-forward svc/frontend 5000:80

# then in another terminal you can do:

% curl http://localhost:5000 # or :3000 for the backend
```

Still, we won't be able to make them talk together for now. That's one the
reasons why we need an ingress controller.

## The Ingress Ctrl

Setting up an Ingress Controller is like installing a doorman in your cluster.
Instead of giving each service a different port (which is quite messy), you
route everything through a single, neat HTTP gateway like:

- http://localhost/ -> frontend
- http://localhost/api -> backend

This is a bit more similar to what you would do in a normal web server, where
you have a single entry point, and then you route everything through that, which
we, cavemen are used to call it a reverse proxy. The ingress controller will
handle all the routing for you, and it will also handle SSL termination, which
is quite nice.

In order to do that we'll need to download the ingress ctrl deployment, we'll
grab from the repository first:

```
curl -L
https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/cloud/deploy.yaml
-o ingress-deployment.yml
```

We'll also need to apply the label to the control plane node, so it can actually
route the traffic to the backend and frontend. This is done by applying the
label `ingress-ready=true` to the control plane node. We already do that at the
cluster config level, but you can do this manually by running:

```bash
% kubectl label node kind-control-plane ingress-ready=true
```

This will tell the ingress controller that this node is ready to handle ingress
traffic. This is a bit specific to `kind`, where you usually would have a
specific worker node for the ingress controller. It'll prevent that the ingress
controller gets overloaded and drop the whole brain of the cluster. For now,
we're good.

Let's finally apply the ingress controller _deployment_ and then our _custom
ingress specs_:

```bash
 [1:39:26] butia ~/s/kind-examples
% kubectl apply -f ingress-deployment.yml
namespace/ingress-nginx created
serviceaccount/ingress-nginx created
serviceaccount/ingress-nginx-admission created
role.rbac.authorization.k8s.io/ingress-nginx created
role.rbac.authorization.k8s.io/ingress-nginx-admission created
clusterrole.rbac.authorization.k8s.io/ingress-nginx created
clusterrole.rbac.authorization.k8s.io/ingress-nginx-admission created
rolebinding.rbac.authorization.k8s.io/ingress-nginx created
rolebinding.rbac.authorization.k8s.io/ingress-nginx-admission created
clusterrolebinding.rbac.authorization.k8s.io/ingress-nginx created
clusterrolebinding.rbac.authorization.k8s.io/ingress-nginx-admission created
configmap/ingress-nginx-controller created
service/ingress-nginx-controller created
service/ingress-nginx-controller-admission created
deployment.apps/ingress-nginx-controller created
job.batch/ingress-nginx-admission-create created
job.batch/ingress-nginx-admission-patch created
ingressclass.networking.k8s.io/nginx created
validatingwebhookconfiguration.admissionregistration.k8s.io/ingress-nginx-admission created

 [1:39:37] butia ~/s/kind-examples
% kubectl get pods -n ingress-nginx
NAME                                        READY   STATUS      RESTARTS   AGE
ingress-nginx-admission-create-bqb5n        0/1     Completed   0          21s
ingress-nginx-admission-patch-txqtk         0/1     Completed   0          21s
ingress-nginx-controller-54d9445ccb-zj8gg   0/1     Pending     0          21s
```

The controller should take a minute or two. Go grab some water or something.
Also, take note of the `-n` flag, which is the namespace. This is important,
because k8s has the concept of namespaces, which is a way to separate resources.
This is quite useful, because you can have multiple clusters running on the same
machine, and you can separate them by namespaces. You can apply that flag mostly
everywhere, and it will work. You can also set a default namespace too.

We can read the logs for the ingress controller to see if everything is working
fine:

```bash
% kubectl logs -n ingress-nginx -f ingress-nginx-controller-54d9445ccb-zj8gg
```

This will show you the logs for the ingress controller, and you can see if
everything is working fine. Let's create our ingress specs now:

```bash
% kubectl apply -f ingress.yml
ingress.networking.k8s.io/app-ingress created
```

This will create the ingress resource, which is the thing that will route the
traffic to the backend and frontend. You can check the status of our ingress by
running:

```bash
% kubectl get ingress
NAME          CLASS    HOSTS       ADDRESS     PORTS   AGE
app-ingress   <none>   localhost   localhost   80      64s
```

There we have it, the address of the ingress controller. Guess what? Our app is
finally up and running. You can check it out by running:

```bash
% curl http://localhost
```

Or, better yet, just access the frontend via your browser. Also, let's curl the
API, we'll be using the `jq` tool to parse the JSON output combined with the
`-s` flag to make it silent. This will make the output a bit more readable:

```bash
% curl -s http://localhost/api/v1/stars | jq
[
  {
    "name": "Sirius",
    "distance": 8,
    "size": "Large"
  },
  {
    "name": "Proxima Centauri",
    "distance": 4,
    "size": "Small"
  },
  {
    "name": "Alpha Centauri",
    "distance": 4,
    "size": "Medium"
  }
]
```

So, for a recap, we have:

- A backend and a frontend, both running in a k8s cluster
- An ingress controller, which is routing the traffic to the backend and
  frontend, acting as a reverse proxy
- A deployment for each of the backend and frontend, which is running the pods

We can now, do some checks, for example, let's assess where each thing is
running, since we defined a couple of worker nodes:

```bash
% kubectl get pods -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP           NODE            NOMINATED NODE   READINESS GATES
backend-7b8ffb9fd8-vxz7v    1/1     Running   0          36m   10.244.1.2   local-worker    <none>           <none>
frontend-5d4784859d-9tl7f   1/1     Running   0          35m   10.244.6.2   local-worker4   <none>           <none>
```

Great, we can see that we have one backend running on the first worker node, and
our frontend was allocated to the fourth worker node. We can also check the
status of the ingress controller by running:

```bash
% kubectl get pods -n ingress-nginx -o wide
NAME                                        READY   STATUS      RESTARTS   AGE   IP           NODE                  NOMINATED NODE   READINESS GATES
ingress-nginx-admission-create-gpdwc        0/1     Completed   0          21m   10.244.4.2   local-worker5         <none>           <none>
ingress-nginx-admission-patch-mrtwj         0/1     Completed   2          21m   10.244.5.3   local-worker6         <none>           <none>
ingress-nginx-controller-54d9445ccb-fpflx   1/1     Running     0          21m   10.244.0.6   local-control-plane   <none>           <none>
```

You can see that the ingress controller is running on the control plane, and the
other, two temporary pods were allocated to the other two worker nodes, but they
are done (see the `Completed` status).
