# backup api

## backup a file
```
POST http://localhost:8080/fa/api/v0/backup
Authorization: Bearer <token>
Content-Type: application/json


# request body
{
    "id": 1,
    "path": "datasets/ansys-out"
}

# response body 200
{
    "code": 2001
    "message: "success"
}


# response body 400
{
    "code": 400
    "message: "err when backup file"
}


```

## get the backup status

```
GET http://localhost:8080/fa/api/v0/backup/1
Authorization: Bearer <token>
Content-Type: application/json

# response body 200
{
    "code": 2001,
    "message": "success",
    "data": {
        "id": 3,
        "path": "/ellis-chen/users/admin/fa",
        "status": "running",
        "stat": {
            "total_size": 0,
            "pack_size": 0
        },
        "extra": ""
    }
}


<!-- status can be running,done,failed,cancel -->

# response body 200
{
    "code": 4001,
    "message": "err when load backup [ 33 ]: no such record",
    "data": null
}

```


## restore a backup
```
POST http://localhost:8080/fa/api/v0/restore
Authorization: Bearer <token>
Content-Type: application/json


# request body
{
    "restore_id": 1,
    "backup_id": 1,
    "dst_path": "datasets/ansys-out-restore"
}

# response body 200
{
    "code": 2001
    "message: "success"
}


# response body 400
{
    "code": 400
    "message: "err when backup file"
}


```

## get the restore status

```
GET http://localhost:8080/fa/api/v0/restore/1
Authorization: Bearer <token>
Content-Type: application/json

# response body 200
{
    "code": 2001,
    "message": "success",
    "data": {
        "restore_id": 1,
        "backup_id": 2,
        "dst_path": "/ellis-chen/users/admin/fa1",
        "status": "success",
        "stat": {
            "total_size": 91457662,
            "pack_size": 0
        },
        "extra": ""
    }
}

<!-- status can be running,done,failed,cancel -->

# response body 200
{
    "code": 4001,
    "message": "err when load restore [ 11 ]: no such record",
    "data": null
}

```

## cancel a backup
```
POST http://localhost:8080/fa/api/v0/backup/cancel
Authorization: Bearer <token>
Content-Type: application/json


# request body
{
    "id": 1,
}

# response body 200
{
    "code": 2001,
    "message": "success",
    "data": null
}


# response body 400
{
    "code": 400
    "message: "err when backup file"
}


```

## cancel all backup
```
POST http://localhost:8080/fa/api/v0/backup/cancel-all
Authorization: Bearer <token>
Content-Type: application/json


# request body
{
    "id": 1,
}

# response body 200
{
    "code": 2001
    "message: "success"
}


# response body 400
{
    "code": 400
    "message: "err when cancel all backup"
}


```

## delete backup repo
```
POST http://localhost:8080/fa/api/v0/backup/destroy-repo
Authorization: Bearer <token>
Content-Type: application/json


# response body 200
{
    "code": 2001
    "message: "success"
}


# response body 400
{
    "code": 400
    "message: "err when destroy repo"
}


```


## delete a backup
```
DELETE http://localhost:8080/fa/api/v0/backup/1
Authorization: Bearer <token>
Content-Type: application/json


# response body 200
{
    "code": 2001
    "message: "success"
}


# response body 400
{
    "code": 400
    "message: "err when delete a backup"
}


```