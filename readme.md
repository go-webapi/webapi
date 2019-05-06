# WebAPI

```
 _       ____  ___    __    ___   _  
\ \    /| |_  | |_)  / /\  | |_) | | 
 \_\/\/ |_|__ |_|_) /_/--\ |_|   |_| 
 
```

> WebAPI 是 CandyFramework v2 的默认底层主机支持。
>
> 借助 WebAPI 我们将 CandyFramework 的规范约束从书面建议变为了编译检查。
>
> 我们很高兴能够与您分享我们的 Go 见解，酷码似飞。

## 承前启后

CandyFramework v1 使用了 Router2 支持底层。虽然 Router 提供了简洁有力的支持，并且在各个方面表现上相当优异，但是因为 Router2 出现了 *CFBA-2019:01 updated 1.01 source code* 缺陷更新并随着框架演进不再与 CandyFramework 的方向高度契合，所以我们立足于几个要点更改了 Router 并为 CandyFramework 提供强劲支持：

1. 自动路由注册

   我们留意到，在以前 Router2 的支撑底层时，端点（*Endpoint*）的注册使用的方法主要有两种：

   1. 含保护的传递注册；
   2. 公开方法的裸注册。

   这两种方法都面临巨大的**手写**注册行为，端点与方法名容易出现不一致。并且后者面临私有领域数据被多重暴露和命名空间重叠的问题。另外，这两种注册行为可能导致同一领域（*Domain*）的业务端点注册到其他父端点的可能，在代码维护上，增大开发者压力。

2. 数据自动协作

   在多次使用 CandyFramework 的实践中，我们发现在操作数据序列化、反序列化的行为上，我们花费了很多的时间，即使提供了中途加密（*Midway Encryption*）套件依然存在大量的序列化检查。在这样的行为上，我们需要思考如何简化这些业务压力。

3. 高灵活中间件

   中间件（*Middleware*）提供的定义是一个函数签名。虽然中间件依然可以提供 `SetMiddleware()` 类似的函数来作为临时装配器，但是不容忽视的是中间件之间的隔离程度过低。我们虽然建议通过 package 来区别中间件，但是因为编写人员的习惯不同，容易造成中间件混杂。我们需要去规范和约束这样的行为以降低这样对于软件质量带来的风险。

## 继往开来

所以我们在 WebAPI 中提供了一系列的革新用来帮助开发者解决种种问题。

### 自动路由注册

我们现在提供这样的路由编写方式：

```go
package controller

type User struct {
	webapi.Controller
}

func (u *User) SayHi() string {
	return "Hi"
}
```

开发者可以直接使用 `return` 语句返回他们预期的回复值。这样即使写到最后忘记写入到上下文也不用出现空回复或者 `panic`。

注册方式很简单：

```go
var host = webapi.NewHost(webapi.Config{})
host.Register("/api", &User{})
```

然后上面的方法将会被自动注册：

```bash
[GET]	/api/User/SayHi
```

然后使用 `curl` 访问它，会得到：

```
Hi
```

的输出内容。

#### 控制器（*Controller*）

在 Router2 中我们遇到了因为扁平化的注册导致的种种问题后，我们重新引入了控制器（*Controller*）。控制器的声明很简单，参考上面的定义我们只需要在结构中引入 `webapi.Controller` 即可。

控制器具有下面的几个默认约定：

- 控制器名是为注册基地址。
- 控制器的第一个返回值在上下文没有返回的时候会被写入到返回正文。
- 控制器内注册点的参数最多为两个，且当参数为两个且没有来源声明的条件下会被默认注册为 `POST`。
- 控制器内注册点的参数为零或者仅一个的时候，在没有参数来源声明的条件下会被默认注册为 `GET`。

但是也允许显式告知：

- 基地址

  可以使用 `RouteAlias() string` 方法来显式指定基地址。例如：

  ```go
  func (u *User) RouteAlias() string {
  	return "usr"
  }
  ```

  方法注册结果为：

  ```bash
  [GET]	/api/usr/SayHi
  ```

- 请求方法

  可以使用 `method[*]` 的方式显式指定请求方法。

  例如：

  ```go
  type User struct {
  	webapi.Controller
  	methodPUT struct{}
  }
  ```

  方法注册结果为：

  ```bash
  [PUT]	/api/usr/SayHi
  ```

  也可以同时写入多个：

  ```go
  type User struct {
  	webapi.Controller
  	methodPUT struct{}
  	methodGET struct{}
  }
  ```

  那么注册结果为：

  ```bash
  [GET]	/api/usr/SayHi
  [PUT]	/api/usr/SayHi
  ```

  > 注意：
  >
  > 显式指定请求方法将会是强制的，回避所有约束的。

- 参数来源指定

  可以使用：

  - fromBody 指定此参数来自正文（*Body*）
  - fromQuery 要求此参数来自查询（*Query*）

  其中，`fromBody` 会导致在没有显示请求方法说明的条件下改变默认的注册行为为 `POST`。

  > 注意：
  >
  > 当参数数量为 2 的时候，第一个参数始终来自于正文。参数来源指定将会无效。

### 前置件（*Predecessor*）

以前的中间件现在变为前置件，通过约定来提高前置件的灵活度并且提高其安全性和复用性。以前需要以一个符合函数签名的函数注册，现在只需要提供符合约定的实例即可。

实例需要提供：

```go
Invoke(*webapi.Context, webapi.HTTPHandler)
```

方法以供运行时注册。

---

最后，欢迎使用 WebAPI 和 CandyFramework，我们彼此酷码似飞。

Finally, Welcome use WebAPI & CandyFramework. Cool to Code.