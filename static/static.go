package static

const CSS = `<style>
*{box-sizing:border-box;margin:0;font-family:Arial,Helvetica,sans-serif,Tahoma,'Microsoft Yahei','Simsun'}
body{font-size:14px}
a{text-decoration:none;}
a:hover{text-decoration:underline}
#container{border-right:dotted 1px #ccc;border-left:dotted 1px #ccc;width:100%;max-width:700px;min-width:320px;margin:0 auto}
#content-0{width:100%}
#content-0 img{display:block;width:100%;}
#content-0 .err{color:red;padding:4px;}
.zebra-false{background:#f1f2f3;}
.title,.snippet{padding:4px 4px 0 4px;border-bottom:dotted 1px #ccc;}
.title .upper{margin:-4px;margin-bottom:0;padding:4px;border-bottom: dotted 1px #ccc;}
.title .id{display: inline-block;margin-right:4px;font-family:consolas,monospace}
.paging {padding:4px}
.paging *{font-family:consolas,monospace}
.paging a{color:#898}
.info{padding:4px 0;font-size:80%;color:#898}
.del{vertical-align: middle;}
.color-blk{display:inline-block;border:solid 1px black;padding:2px;width:24px;height:24px;line-height:20px;text-align:center}
.color-blk span{color:violet}
#post-form{border-collapse:collapse}
#post-form td{border: dotted 1px #ccc;border-left: none;border-right: none;}
#post-form, #post-form .ctrl{resize:vertical;width:100%;max-width:100%;min-width:100%;border:none}
#post-form .ctrl{display:block}
#post-form input.ctrl{padding:4px;}
#post-form input[type=radio] + label{margin-right:4px}
#post-form .title{padding: 4px;white-space:nowrap;width:1px;text-align:right}
.bar-item{color:white;display:inline-block;zoom:1;*display:inline;margin:-4px 0;padding:4px 8px;border-left:solid 1px #889}
.header,.footer{background:#667;padding:4px 0;color:white;margin:0 -1px}
</style>`

const UntitledSnippet = "无标题"

const NewSnippet = "新片段"

const AllSnippets = "浏览"

const NoPermission = "无权限删除改片段"

const SnippetNotFound = "未找到该片段，其可能已被删除或失效"

const InternalError = "内部错误"

const CooldownTime = "请不要短时间内发布大量内容"

const EmptyContent = "空内容或内容过长"

const Back = "后退"

const Delete = "删除"

const Error = "错误"

const NewSnippetForm = `<form method=POST action=/post><table id=post-form>
<tr><td colspan=4 style="font-size:1.5em;text-align:center;padding:4px">Converteix text a la imatge amb un sol clic</td></tr>
<tr>
	<td class=title>标题</td><td><input class=ctrl name=title placeholder="` + UntitledSnippet + `"></td>
	<td class=title style="border-left:dotted 1px #ccc">发布者</td><td><input class=ctrl name=author placeholder="N/A"></td>
</tr>
<tr><td colspan=4><textarea class=ctrl name=content rows=10 style="padding:4px" placeholder="内容 (text)"></textarea></td></tr>
<tr><td colspan=4 style="padding:4px">
<div style="line-height:2em">有效期:
<input id=ttl1 type=radio name=ttl value="86400">
<label for=ttl1>1天</label>
<input id=ttl2 type=radio name=ttl value="604800">
<label for=ttl2>1周</label>
<input id=ttl3 type=radio name=ttl value="2592000">
<label for=ttl3>30天</label>
<input id=ttl4 type=radio name=ttl value="0" checked>
<label for=ttl4>永久</label>
<input type=submit value="发布" style="float:right">
</div>
<div style="line-height:2em">颜色:
<input id=theme1 type=radio name=theme value=white checked>
<label for=theme1 class=color-blk style="background:white;color:black;">A<span>+</span></label>
<input id=theme2 type=radio name=theme value=black>
<label for=theme2 class=color-blk style="background:black;color:white;">A<span>+</span></label>
<input id=theme3 type=radio name=theme value=purewhite>
<label for=theme3 class=color-blk style="background:white;color:black;">A</label>
<input id=theme4 type=radio name=theme value=pureblack>
<label for=theme4 class=color-blk style="background:black;color:white;">A</label>
</div>
<div>
<ol>
<li>每行文本可能会被插入多个空格以保证与80列对齐
<li>若不想被空格破坏格式（如代码），请插入一对三个反引号（单独一行）：
	<p style="font-family:consolas,monospace">` + "```" + `<br>&nbsp;&nbsp;&nbsp;&nbsp;a = b + c; <br>` + "```" + `</p>
</ol>
</div>
</td></tr>
</table>
</form>`

const Header = `<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=1.0, minimum-scale=1.0, maximum-scale=1.0">
<meta charset="utf-8">
` + CSS + `
<div id=container>
<div class=header>
<a class=bar-item href=/>` + NewSnippet + `</a><!--
--><a class=bar-item href=/list>` + AllSnippets + `</a>
</div><div id=content-0>`

const Footer = `</div><div class=footer><!--
--><span class=bar-item>%s</span><!--
--><span class=bar-item>%d snippets</span><!--
--><span class=bar-item>%d blocks</span><!--
--><span class=bar-item>%0.2f%% cap</span><!--
--><a class=bar-item href="https://github.com/coyove/eighty">Github</a>
</div></div>`
