package static

const CSS = `<style>
*{box-sizing:border-box;margin:0}
body{font-size:14px;font-family:Arial,Helvetica,sans-serif}
a{text-decoration:none;}
a:hover{text-decoration:underline}
#container{border-right:dotted 1px #ccc;border-left:dotted 1px #ccc;width:100%;max-width:700px;margin:0 auto}
#content-0{width:100%}
#content-0 img{display:block;width:100%;}
#content-0 .err{color:red;padding:4px;}
.zebra-false{background:#eee}
.title,.snippet{padding:4px}
.title .upper{margin:-4px;margin-bottom:4px;padding:4px;border-bottom: dotted 1px #ccc;}
.title .id{display: inline-block;margin:-4px 4px -4px -4px;padding:4px;border-right: dotted 1px #ccc;font-family:consolas,monospace}
.snippet{border-bottom:dotted 1px #ccc;}
.paging *{font-family:consolas,monospace}
.paging a{color:#898}
.info{padding:4px 0;font-size:80%;color:#898}
#post-form{border-collapse:collapse}
#post-form td{padding: 4px;border: dotted 1px #ccc;border-left: none;border-right: none;}
#post-form, #post-form .ctrl{resize:vertical;width:100%;max-width:100%;min-width:100%;border:none}
#post-form .ctrl{display:block}
#post-form .title{white-space:nowrap;width:1px;text-align:right}
.header a,
.footer a,
.header span,
.footer span{color:white;display:inline-block;zoom:1;*display:inline;padding:0 2px}
.header .sep,.footer .sep{color:#ccc}
.header,.footer{background:#667;padding:4px;color:white;margin:0 -1px}
</style>`

const UntitledSnippet = "无标题"

const NewSnippet = "新片段"

const AllSnippets = "浏览"

const NoPermission = "无权限删除改片段"

const SnippetNotFound = "未找到该片段，其可能已被删除或失效"

const InternalError = "内部错误"

const EmptyContent = "无法提交空内容"

const Back = "后退"

const Delete = "删除"

const Error = "错误"

const NewSnippetForm = `<form method=POST action=/post><table id=post-form>
<tr>
	<td class=title>标题</td><td><input class=ctrl name=title placeholder="` + UntitledSnippet + `"></td>
	<td class=title style="border-left:dotted 1px #ccc">发布者</td><td><input class=ctrl name=author placeholder="N/A"></td>
</tr>
<tr><td colspan=4 style="padding:0"><textarea class=ctrl name=content rows=10></textarea></td></tr>
<tr><td colspan=4 style="border-bottom:none">有效期:
<select name=ttl>
<option value="3600">1小时</option>
<option value="86400">1天</option>
<option value="2592000">30天</option>
<option value="0" selected>永久</option>
</select>&nbsp;
颜色:
<select name=theme>
<option value="black">black</option>
<option value="pureblack">black2</option>
<option value="purewhite">white2</option>
<option value="white" selected>white</option>
</select>
<input type=submit value="发布" style="float:right"></td></tr>
</table>
</form>
<div style="margin:4px 0;font-size:0.9em">
<ol>
<li>每行文本可能会被插入多个空格以保证与80列对齐
<li>若不想被空格破坏格式（如代码），请使用一对三反引号包起：` + "```" + ` &lt;your code...&gt; ` + "```" + `；
</ol>
</div>`
