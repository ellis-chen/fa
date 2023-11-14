#!/usr/bin/env python3
# -*- coding:utf-8 -*-

import argparse
import json
import os
from http import client

apiHost = os.getenv("API_HOST", 'fcc-dev.ellis-chentech.com')
apiPort = os.getenv("API_PORT", 80)
apiToken = os.getenv("API_TOKEN")


def read_as_json(path: str):
    with open(path, 'r', encoding='utf-8') as f:
        return json.load(f)


def open_conn():
    return client.HTTPSConnection(host=apiHost)


def extra_resp(connection):
    resp = connection.getresponse()
    result_str = resp.read().decode("utf-8")
    print("http resp is {result_str}".format(result_str=result_str))
    result = json.loads(result_str)

    if resp.getcode() != 200:
        raise RuntimeError("Http error {result}".format(result=result))
    if result["code"] != 200:
        raise RuntimeError("bad response code {code}, message is {message}".format(**result))
    else:
        return result['data'] if 'data' in result else {}


def parser_login(parser):
    def handle(args):
        print(f'args.username is {args.username}, args.password is {args.password}')
        connection = open_conn()
        connection.request(
            "POST",
            '/user/api/users/login',
            body=json.dumps({'usernameOrEmail': args.username, 'password': args.password}),
            headers={
                "Content-Type": "application/json",
            }
        )
        result = extra_resp(connection)
        print(result['accessToken'])

    # {"usernameOrEmail":"u15629067656","password":"U87MDWBVN1RmFzdG9uZTEhRPJG6UIHMI"}
    login = parser.add_parser('login', help="login and get a token")
    login.add_argument("-u", '--username', dest='username', required=True, help="user name",
                       default=True)
    login.add_argument("-p", '--password', dest='password', required=True, help="password",
                       default=True)
    login.set_defaults(func=handle)


def parser_create(parser):
    def handle(args):
        connection = open_conn()
        connection.request(
            method="POST",
            headers={
                "Content-Type": "application/json",
                "Authorization": f'Bearer {apiToken}'
            },
            url="/fa/api/v0/internal/activate-app",
            body=json.dumps({"app_name": args.app})
        )
        result = extra_resp(connection)
        print(result)
        print(f"------- success activating app  name={args.app})")

    create = parser.add_parser('create', help="activate an app")
    create.add_argument("-a", '--app', dest='app', required=True, help="app name",
                        default=True)
    create.set_defaults(func=handle)


def parser_deactivate(parser):
    def handle(args):
        connection = open_conn()
        connection.request(
            method="POST",
            headers={
                "Content-Type": "application/json",
                "Authorization": f'Bearer {apiToken}'
            },
            url="/fa/api/v0/internal/deactivate-app",
            body=json.dumps({"app_name": args.app})
        )
        result = extra_resp(connection)
        print(json.dumps(result))
        print(f"------success deactivate the app [ {args.app} ]")

    approval = parser.add_parser('deactivate', help="deactivate the app")
    approval.add_argument("-a", '--app', dest='app', required=True, help="app name",
                          default=True)
    approval.set_defaults(func=handle)


# python3 fa.py login -u u15629067656 -p U87MDWBVN1RmFzdG9uZTEhRPJG6UIHMI
def main():
    top_parser = argparse.ArgumentParser()
    top_parser.set_defaults(func=lambda a: top_parser.print_usage())
    sub_parser = top_parser.add_subparsers()
    parser_login(sub_parser)
    parser_create(sub_parser)
    parser_deactivate(sub_parser)
    args = top_parser.parse_args()
    args.func(args)
    return True


if __name__ == '__main__':
    import sys

    sys.exit(not main())
