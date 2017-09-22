# cse
零编码,纯配置的通用(Common)搜索(Search)引擎(Engine).  
cse使用[goose](https://github.com/getwe/goose)作为检索框架,实现检索策略.

## 使用方法
### 建库
    ./cse -b -d pathToDataFile

### 启动检索模式(默认)
    ./cse
    
### 检索
#### 使用命令行工具发起检索请求
先安装[goose-qtool](https://github.com/getwe/goose-qtool),支持go安装第三方库的方法

    go get github.com/getwe/goose-qtool
发起检索

    goose-qtool -i 127.0.0.1 -p 7788 -c '{"query":"test"}'
    
#### 使用前端调试页面
为了更方便调试,我又开发了调试前端,具体部署使用方法详见[csedebug](https://github.com/getwe/csedebug)

## 文档输入要求

### 被索引doc要求
cse要求输入的每一个doc是一个合法的json结构体.必须包含的字段有:

#### cse_docid
可唯一标识这个doc的外部ID,要求uint32类型
#### cse_value
> goose框架提供一个[]byte,可以在ranking阶段方便获取.至于这块buffer怎么使用完全有策略定制.  

cse把value[0:4]这前四个字节用于存储clusterid,clusterid用于结果类聚.  
cse把剩余空间value[5:]用于存储调权字段.
调权字段,用于非文本加权.要求uint8数组.必须用uint8类型是以为内部采用一个byte来存储每一个元素,一般全部采用取值[0,100].  
默认取cse_value的第一个数字为clusterid,剩余的数字为调权id.  
如果不需要`类聚`功能,那么直接把clusterid设置为cse_docid,该功能就自动失效.
#### cse_maintitle
doc的核心maintitle.
#### cse_title
doc的普通title.maintitle跟title的索引逻辑是一致的,但是可以在配置中设置不同的权重.
#### cse_keyword
cse中,title完全匹配不一定就是相关性最好的,也有可能是keyword相关.一个doc支持输入keyword列表,表示该doc的keyword,每一个keyword都可以带有不同的权重,取值范围是0到1.
#### cse_data
由输入完全自定义的数据.
#### 合法的输入实例
    {
    	"cse_docid" : 23333,
    	"cse_value" : [23333,80,70,64,55,30,20],
        "cse_maintitle" : ["广东","粤"],
        "cse_title" : ["广东省","岭南"],
        "cse_keyword" : "[ {"kw":"美食","boost":1.0},{"kw":"经济","boost":0.8} ],
        "cse_data" : {},
    }
cse这样来理解这个输入doc:

* cse_docid表示外部id是23333
* cse_value表示类聚字段clusterid为23333,6个调权字段是80,70,64,55,30,20
* cse_maintitle表示文档有两个核心title,分别是"广东"和"粤"
* cse_title表示文档有两个普通title,分别是"广东"和"岭南"
* cse_keyword表示文档有多个关键字,每个关键字还带有一个置信度,取值[0.0,1.0].title相对于默认置信度为1.0
* cse_data整个包会作为最终的结果的包体,可以是任意的合法json包
    
这里为了阅读方便,整个json包写成多行.实际使用,由于cse读取磁盘索引是每读取一行认为是一个doc,所以同一个doc需要压缩成一行,即每个doc在文件中用`'\n'`隔开.

