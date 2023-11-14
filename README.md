# fa

fa is a Compute-Cloud subproject which aims at providing a unified file access for ellis-chen users

## get started

```shell
$ git submodule update --recursive
$ git submodule foreach "(git checkout master; git pull)&"

# install the required plugins to generate protos
$ go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
$ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# make sure GOPATH in PATH
$ export PATH="$PATH:$(go env GOPATH)/bin"
# update the proto file
$ make proto

# install locally
$ make install

$ docker-compose up -d

# install rclone
$ wget -qO - http://nexus.ellis-chentech.com/repository/ellis-chen-raw/gpg/public.gpg.key | sudo apt-key add -
$ echo "deb [trusted=yes] http://nexus.ellis-chentech.com/repository/ellis-chen-apt impish main" | sudo tee /etc/apt/sources.list.d/rclone.list > /dev/null
$ sudo apt update && sudo apt install rclone

# use rclone to test fa connectivity
$ export RCLONE_CONFIG=$(pwd)/build/rclone.conf
# list the root path: :) should contains at least 3 files (.bash_logout, .bash_profile, .bashrc)
$ rclone ls fa:

```

## test backup

### setup server
```shell
# test oracle oci backup
sudo FA_JWT_SECRET=JWTSuperSecretKey  FB_LOC_REGION=ap-tokyo-1 FB_LOC_VENDOR=oracle AWS_ACCESS_KEY_ID=<oracle ak> AWS_SECRET_ACCESS_KEY=<oracle sk>  fa serve

# test aws s3 backup
sudo FA_JWT_SECRET=JWTSuperSecretKey AWS_ACCESS_KEY_ID=<s3 ak>  AWS_SECRET_ACCESS_KEY=<s3 sk> fa serve
```

### setup client

```shell
export TOKEN="<use a fcce token>"

# new backup with backupID=1, path=dir1
curl -X POST 'http://localhost:8080/fa/api/v0/backup/new' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'  -d '{"id": 1, "path": "dir1" }'

# get backup status of backupID=1
curl -X GET 'http://localhost:8080/fa/api/v0/backup/1' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'

# new restore with restoreID=1, backupID=1, dst_path=dir2
curl -X POST 'http://localhost:8080/fa/api/v0/restore/new' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'  -d '{"restore_id": 1, "backup_id": 1, "dst_path": "dir2" }'

# get backup status of restoreID=1
curl -X GET 'http://localhost:8080/fa/api/v0/restore/1' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'

# cancel a backup with backupID=1
curl -X POST 'http://localhost:8080/fa/api/v0/backup/cancel' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'  -d '{"id": 1 }'

# cancel a restore with restoreID=1
curl -X POST 'http://localhost:8080/fa/api/v0/restore/cancel' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'  -d '{"id": 1 }'

# delete a backup with id=1
curl -X DELETE 'http://localhost:8080/fa/api/v0/backup/1' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'

# terminate the backup repository (will delete everything on remote, use cautionly)
curl -X POST 'http://localhost:8080/fa/api/v0/backup/destroy-repo' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json'


```

### full restore

> Before restore, you should choose the restore directoy wisely. Because fa is running inside k8s pod, and pod has its
> resource limit. So it's wise to set dst to /ellis-chen/restore

```shell
vagrant@station:~$ export TENANT_ID="1622436422965399552"
vagrant@station:~$ kubectl --kubeconfig kubeconfig   --insecure-skip-tls-verify -n $TENANT_ID get po -l app=fa
NAME                  READY   STATUS                   RESTARTS   AGE
fa-bd5f77c9b-ghcwd    1/1     Running                  0          23m

vagrant@station:~$ kubectl --kubeconfig kubeconfig   --insecure-skip-tls-verify -n $TENANT_ID exec -it fa-bd5f77c9b-ghcwd bash
# using curl to restore the full auto backup into /ellis-chen/restore directory
# the curl will block until the restore succed
fa-bd5f77c9b-ghcwd:/restore# nohup curl 'http://localhost/fa/public/full-restore?dst=/ellis-chen/restore' &

```

### debug with container

```shell

docker run -it --rm -p 2345:2345 -p 8080:80 -e FA_JWT_SECRET="JWTSuperSecretKey" hub.ellis-chentech.com/cce/fa:master-dev

docker run -it --rm --name fa -p 2345:2345 -p 8080:80 -e FA_DEBUG="true" -e FA_JWT_SECRET="JWTSuperSecretKey" -e AWS_ACCESS_KEY_ID="xxxx" -e AWS_SECRET_ACCESS_KEY="xxx" hub.ellis-chentech.com/infra/fa:v1.2.1

```

### debug with docker-compose container

```shell
# export the 9090 port
iptables -t nat -A DOCKER ! -i br-08e911c8de15 -p tcp -m tcp --dport 9090 -j DNAT --to-destination 172.21.0.2:9090
iptables -A DOCKER ! -i br-08e911c8de15 -o br-08e911c8de15 -d 172.21.0.2 -p tcp -m tcp --dport 9090 -j ACCEPT
# export the 8080 port
iptables -t nat -A DOCKER ! -i br-08e911c8de15 -p tcp -m tcp --dport 8080 -j DNAT --to-destination 172.21.0.2:8080
iptables -A DOCKER ! -i br-08e911c8de15 -o br-08e911c8de15 -d 172.21.0.2 -p tcp -m tcp --dport 8080 -j ACCEPT
# export the 3000 port
iptables -t nat -A DOCKER ! -i br-08e911c8de15 -p tcp -m tcp --dport 3000 -j DNAT --to-destination 172.21.0.2:3000
iptables -A DOCKER ! -i br-08e911c8de15 -o br-08e911c8de15 -d 172.21.0.2 -p tcp -m tcp --dport 3000 -j ACCEPT
```

### debug with kopia

```shell
# connect to remote backup to check backup status
kopia repository connect s3 --bucket kopia-backup-center --access-key xxx --secret-access-key yyy --region bj --endpoint s3.bj.bcebos.com --prefix 1662005120340201472/  --config-file `pwd`/kopia.config
```