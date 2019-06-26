# WebAPI

```
 _       ____  ___    __    ___   _  
\ \    /| |_  | |_)  / /\  | |_) | | 
 \_\/\/ |_|__ |_|_) /_/--\ |_|   |_| 
 
```

 [English](https://github.com/johnwiichang/webapi/blob/master/readme.md) | 中文

WebAPI 是一个适用于 Golang 的 Web API 服务开发的基础库。使用 WebAPI 可以有效降低编码出错的概率并回避无聊的重复代码。WebAPI for Golang 灵感来源于 Microsoft ASP.NET Core，所以如果对于 .NET 或者 JavaEE 熟悉的开发者而言，定会游刃有余。

## 能力

### 自动路由注册与复合控制器

**聚焦业务核心回避无关的规则，DDD 与 MVVM 等设计模式高度相容。不同业务模块拥有自身控制器（组），支持模块细分与多模块统一整合的设计（允许多人协同）。基于约定的自动路由注册降低冗余代码，提高设计效率同时回避公开地址与内部实现失去同步的可能。**

声明控制器：

```go
type Article struct {
	webapi.Controller
}
```

声明接入点：

```go
func (article *Article) Show(query struct {
  GUID string `json:"guid"`
}) string {
	return fmt.Sprintf("you are reading post-%s", query.GUID)
}
```

> WebAPI 遵循 Golang 原则，但凡可访问的方法（大写字母开头的函数）均会被注册为 API 接入点。

接入点会被注册为 `/article/show?guid=[guid]`，如果有多个控制器处理不同业务，那么可以通过 `RouteAlias() string` 方法来指定控制器别名：

```go
type article struct {
	webapi.Controller
	id uint
}

func (article *article) RouteAlias() string {
	return "article"
}
```

然后不管在 `article` 还是 `Article` 下的方法均会注册到 `/article` 下。不过需要注意的是，此类注册器需要回避重名问题。不过在运行时 WebAPI **不**会针对重名方法做出致命警告 (panic)。

> 使用时，不妨将各个业务模块细分给不同的人去完成。最后通过 `RouteAlias` 可以轻松将他们整合在一起。

### 查询/正文自动序列化与反序列化支持

**完全消除在业务逻辑中的正文读取、序列化/反序列化、查询检索与转化，借助于中间件，甚至还可以配置 MsgPack 等非系统内建/私有化序列器。**

我们使用  `curl` 访问一下刚才的 API：

```bash
~ curl http://localhost:9527/article/show\?guid\=79526
#you are reading post-79526
```

可以看到参数自动放到了 `query` 中。亦可支持正文自动序列化，例如声明方法：

```go
func (article *article) Save(entity *struct {
	ID         uint
	Title      string
	Content    string
	CreateTime time.Time
}, query struct {
	CreateTime string `json:"time"`
}) {
	entity.CreateTime, _ = time.Parse("2006-01-02", query.CreateTime)
	entity.ID = article.id
	article.Reply(http.StatusAccepted, entity)
}
```

这个方法将会注册为 `[POST]  /article/{digits}/save`，因为存在 `*struct{}` 结构，所以默认为 POST，但是可以通过 `method[HTTPMETHOD] struct` 的私有字段的形式去显式声明 HTTP 方法。同样使用 `curl`：

```go
~ curl -X "POST" "http://localhost:9527/article/123/save?time=2019-01-01" \
     -H 'Content-Type: application/json; charset=utf-8' \
     -d $'{
  "Title": "Hello WebAPI for Golang",
  "Content": "Awesome!"
}'

#[{"ID":123,"Title":"Hello WebAPI for Golang","Content":"Awesome!","CreateTime":"2019-01-01T00:00:00Z"}]
```

可以看到查询中的时间成功被访问并赋到了正文。也请留意到，不管是之前使用的 `string` 作为返回值，还是这个节点的，手动使用 `.Reply(STATUSCODE, INTERFACE{})` 都可以自动处理并回复给客户端。

> 序列化器可以手动指定。在上下文 (Context) 的 Serializer 属性中指定。

同时，查询和正文的结构支持检查，为他们添加 `Check() error` 方法即可在进入业务代码之前检查数据的合法性，将防范性编码与业务隔离开来。

### 路由前置条件（前参数化访问）支持

**收束控制器处理数据范畴设立 API 访问准入门槛。提供其他路由服务无法提供的匹配-回落和具体业务控制器前置条件能力，从根本隔离开非法访问，降低出错几率，提高业务编码效率并提高系统鲁棒性。**

刚才的请求中我们看到，访问地址 `/article/123/save` 中的 `123` 被捕获并且最后在回复正文的 `ID` 中出现。WebAPI 允许为控制器设立前置条件（Precondition），声明的方法：

```go
func (article *article) Init(id uint) (err error) {
	article.id = id
	return
}
```

只需要为控制器声明返回值为 `error` 且名称为 `Init` 的方法即可自动在进入实际方法前调用它。节点注册形式也发生了些许变更。如果参数为

- 整型、长整型、无符号整型、无符号长整型（Int/UInt）那么将会得到一个 `/{digits}` 的注册点
- 单精度浮点、双精度浮点（Float32/64）那么将会得到一个 `/{float}` 的注册点
- 字符串（String）将会得到一个 `/{string}` 的注册点

所以上面的 `Init(uint) error` 函数将会产生 `/{digits}` 的注册点。

如果调用函数返回的错误值不为空，那么将会通知客户端 Bad Request。这从侧面区分了对象方法（Object Method）和静态方法（Static Method），编码时可以更关注业务本身而不用去操心各种前置条件审查，或节省大量近似的代码。

### 端点条件（后参数化访问）支持

**不需要反向代理配置伪静态即可提供原生支持参数化访问，提供更直观简洁的 API。**

既然前参数化访问都支持，那么自然后参数化访问也可以。刚才我们遇到，访问文章正文需要使用查询参数，虽然可以正常工作，但是未免显得太过单调。通过前置参数支持需要使用 `Read` 一类的方法，感觉不自然。我们可以通过后置参数的形式来提供形如 `/article/{guid}` 的访问形式：

```go
func (article *Article) Index(guid string) string {
	return fmt.Sprintf("you are reading post-%s", guid)
}
```

使用 `curl` 测试一下：

```go
~ curl http://localhost:9527/article/id-233666
#you are reading post-id-233666
```

> ⚠️ 注意
>
> 此方法也可以通过 `func (article *article) Index(id int)` 的方法实现。在本例中两个方法允许共存，因为前者为 `/article/{string}` 后者为 `/article/{digits}`。在协作的时候务必注意此类问题。如果出现重复注册节点，控制器将会注册失败并提示错误。

### 中途加密策略支持

**完全可托付的原生加密解密，兼容密钥协商机制，即使密钥变更或不唯一，只需一次设定，流水线明文密文处理更加可靠，完全杜绝因为疏忽或者意外造成的机密元数据泄露的可能，数据安全高枕无忧。**

加解密服务依托于上下文，可以在中间件中指定上下文中的 `CryptoService` 来实现提供统一的加解密服务，将无关业务的加解密方法独立出去，提高开发者效率。

方法亦可在使用中途更改，即此加密解密的模块是动态可替换的。

## 性能

在 8 vCPU / 16G RAM 的测试环境 TLinux 虚拟机（非空闲）上使用 Cyborg 性能实用程序从 100 客户端，200 请求发起压力到 580 客户端，1160 请求的 Hello World 接口性能记录：

| 客户端数 | 总请求 | 总响应时长 | 秒级处理量 |
| -------- | ------ | ---------- | ---------- |
| 100      | 200    | 0.011942s  | 8374.09354 |
| 120      | 240    | 0.010043s  | 11949.1254 |
| 140      | 280    | 0.008378s  | 16710.2346 |
| 160      | 320    | 0.012892s  | 12410.937  |
| 180      | 360    | 0.011250s  | 15999.7355 |
| 200      | 400    | 0.011609s  | 17228.3244 |
| 220      | 440    | 0.017484s  | 12582.8826 |
| 240      | 480    | 0.020186s  | 11889.3559 |
| 260      | 520    | 1.004284s  | 258.890958 |
| 280      | 560    | 0.015453s  | 18119.6865 |
| 300      | 600    | 0.016319s  | 18383.022  |
| 320      | 640    | 1.011559s  | 316.343278 |
| 340      | 680    | 1.014014s  | 335.301017 |
| 360      | 720    | 0.016673s  | 21591.3134 |
| 380      | 760    | 0.020759s  | 18305.5029 |
| 400      | 800    | 1.007692s  | 396.946831 |
| 420      | 840    | 0.024735s  | 16980.1533 |
| 440      | 880    | 0.023144s  | 19011.1727 |
| 460      | 920    | 0.025437s  | 18083.5409 |
| 480      | 960    | 1.006551s  | 476.875941 |
| 500      | 1000   | 1.008704s  | 495.685546 |
| 520      | 1040   | 1.010670s  | 514.510124 |
| 540      | 1080   | 1.009176s  | 535.089889 |
| 560      | 1120   | 0.034008s  | 16466.9452 |
| 580      | 1160   | 0.031361s  | 18494.0912 |

由于目标机器以及压力发起机器均为虚拟机的缘故，呈现出数据抖动，根据实际经验上，物理服务器上此抖动不存在。整体性能中位数 12582.88259（1.2w），摒弃低于 1000 的记录性能中位数 16980.15331（1.7w）。对于单节点而言性能相对乐观。

---

最后，欢迎使用 WebAPI，我们彼此酷码似飞。

Finally, Welcome use WebAPI. Cool to Code.