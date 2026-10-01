package lit

// HTML tag and attribute tables for binary content documents, transcribed
// from the format reference tables (ids and per-tag maps are functional
// data). Tag ids are sparse by design; gaps decode no tag.
//
// Note on embed 0x8BBF: the C reference lists "codebase" first (winning its
// linear scan) with "src" second, while the Python reference keeps only
// "src" (its mapping cannot hold duplicates, and the "codebase" entry is
// commented out there as a deliberate choice). This port follows the Python
// reference: "src" decodes, which also yields the better HTML output.

var htmlTags = [...]string{
	0:   "",
	1:   "",
	2:   "",
	3:   "a",
	4:   "acronym",
	5:   "address",
	6:   "applet",
	7:   "area",
	8:   "b",
	9:   "base",
	10:  "basefont",
	11:  "bdo",
	12:  "bgsound",
	13:  "big",
	14:  "blink",
	15:  "blockquote",
	16:  "body",
	17:  "br",
	18:  "button",
	19:  "caption",
	20:  "center",
	21:  "cite",
	22:  "code",
	23:  "col",
	24:  "colgroup",
	25:  "",
	26:  "",
	27:  "dd",
	28:  "del",
	29:  "dfn",
	30:  "dir",
	31:  "div",
	32:  "dl",
	33:  "dt",
	34:  "em",
	35:  "embed",
	36:  "fieldset",
	37:  "font",
	38:  "form",
	39:  "frame",
	40:  "frameset",
	41:  "",
	42:  "h1",
	43:  "h2",
	44:  "h3",
	45:  "h4",
	46:  "h5",
	47:  "h6",
	48:  "head",
	49:  "hr",
	50:  "html",
	51:  "i",
	52:  "iframe",
	53:  "img",
	54:  "input",
	55:  "ins",
	56:  "kbd",
	57:  "label",
	58:  "legend",
	59:  "li",
	60:  "link",
	61:  "tag61",
	62:  "map",
	63:  "tag63",
	64:  "tag64",
	65:  "meta",
	66:  "nextid",
	67:  "nobr",
	68:  "noembed",
	69:  "noframes",
	70:  "noscript",
	71:  "object",
	72:  "ol",
	73:  "option",
	74:  "p",
	75:  "param",
	76:  "plaintext",
	77:  "pre",
	78:  "q",
	79:  "rp",
	80:  "rt",
	81:  "ruby",
	82:  "s",
	83:  "samp",
	84:  "script",
	85:  "select",
	86:  "small",
	87:  "span",
	88:  "strike",
	89:  "strong",
	90:  "style",
	91:  "sub",
	92:  "sup",
	93:  "table",
	94:  "tbody",
	95:  "tc",
	96:  "td",
	97:  "textarea",
	98:  "tfoot",
	99:  "th",
	100: "thead",
	101: "title",
	102: "tr",
	103: "tt",
	104: "u",
	105: "ul",
	106: "var",
	107: "wbr",
	108: "",
}

var htmlAttrs = map[int]string{
	0x8010: "tabindex",
	0x8046: "title",
	0x804B: "style",
	0x804D: "disabled",
	0x83EA: "class",
	0x83EB: "id",
	0x83FE: "datafld",
	0x83FF: "datasrc",
	0x8400: "dataformatas",
	0x87D6: "accesskey",
	0x9392: "lang",
	0x93ED: "language",
	0x93FE: "dir",
	0x9771: "onmouseover",
	0x9772: "onmouseout",
	0x9773: "onmousedown",
	0x9774: "onmouseup",
	0x9775: "onmousemove",
	0x9776: "onkeydown",
	0x9777: "onkeyup",
	0x9778: "onkeypress",
	0x9779: "onclick",
	0x977A: "ondblclick",
	0x977E: "onhelp",
	0x977F: "onfocus",
	0x9780: "onblur",
	0x9783: "onrowexit",
	0x9784: "onrowenter",
	0x9786: "onbeforeupdate",
	0x9787: "onafterupdate",
	0x978A: "onreadystatechange",
	0x9790: "onscroll",
	0x9794: "ondragstart",
	0x9795: "onresize",
	0x9796: "onselectstart",
	0x9797: "onerrorupdate",
	0x9799: "ondatasetchanged",
	0x979A: "ondataavailable",
	0x979B: "ondatasetcomplete",
	0x979C: "onfilterchange",
	0x979F: "onlosecapture",
	0x97A0: "onpropertychange",
	0x97A2: "ondrag",
	0x97A3: "ondragend",
	0x97A4: "ondragenter",
	0x97A5: "ondragover",
	0x97A6: "ondragleave",
	0x97A7: "ondrop",
	0x97A8: "oncut",
	0x97A9: "oncopy",
	0x97AA: "onpaste",
	0x97AB: "onbeforecut",
	0x97AC: "onbeforecopy",
	0x97AD: "onbeforepaste",
	0x97AF: "onrowsdelete",
	0x97B0: "onrowsinserted",
	0x97B1: "oncellchange",
	0x97B2: "oncontextmenu",
	0x97B6: "onbeforeeditfocus",
}

var htmlATTRS3 = map[int]string{
	0x0001: "href",
	0x03EC: "target",
	0x03EE: "rel",
	0x03EF: "rev",
	0x03F0: "urn",
	0x03F1: "methods",
	0x8001: "name",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS5 = map[int]string{
	0x9399: "clear",
}
var htmlATTRS6 = map[int]string{
	0x8001: "name",
	0x8006: "width",
	0x8007: "height",
	0x804A: "align",
	0x8BBB: "classid",
	0x8BBC: "data",
	0x8BBF: "codebase",
	0x8BC0: "codetype",
	0x8BC1: "code",
	0x8BC2: "type",
	0x8BC5: "vspace",
	0x8BC6: "hspace",
	0x978E: "onerror",
}
var htmlATTRS7 = map[int]string{
	0x0001: "href",
	0x03EA: "shape",
	0x03EB: "coords",
	0x03ED: "target",
	0x03EE: "alt",
	0x03EF: "nohref",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS8 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS9 = map[int]string{
	0x03EC: "href",
	0x03ED: "target",
}
var htmlATTRS10 = map[int]string{
	0x938B: "color",
	0x939B: "face",
	0x93A3: "size",
}
var htmlATTRS12 = map[int]string{
	0x03EA: "src",
	0x03EB: "loop",
	0x03EC: "volume",
	0x03ED: "balance",
}
var htmlATTRS13 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS15 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS16 = map[int]string{
	0x07DB: "link",
	0x07DC: "alink",
	0x07DD: "vlink",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938A: "background",
	0x938B: "text",
	0x938E: "nowrap",
	0x93AE: "topmargin",
	0x93AF: "rightmargin",
	0x93B0: "bottommargin",
	0x93B1: "leftmargin",
	0x93B6: "bgproperties",
	0x93D8: "scroll",
	0x977B: "onselect",
	0x9791: "onload",
	0x9792: "onunload",
	0x9798: "onbeforeunload",
	0x97B3: "onbeforeprint",
	0x97B4: "onafterprint",
	0xFE0C: "bgcolor",
}
var htmlATTRS17 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS18 = map[int]string{
	0x07D1: "type",
	0x8001: "name",
}
var htmlATTRS19 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x93A8: "valign",
}
var htmlATTRS20 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS21 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS22 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS23 = map[int]string{
	0x03EA: "span",
	0x8006: "width",
	0x8049: "align",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS24 = map[int]string{
	0x03EA: "span",
	0x8006: "width",
	0x8049: "align",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS27 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938E: "nowrap",
}
var htmlATTRS29 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS31 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938E: "nowrap",
}
var htmlATTRS32 = map[int]string{
	0x03EA: "compact",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS33 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938E: "nowrap",
}
var htmlATTRS34 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS35 = map[int]string{
	0x8001: "name",
	0x8006: "width",
	0x8007: "height",
	0x804A: "align",
	0x8BBD: "palette",
	0x8BBE: "pluginspage",
	0x8BBF: "src",
	0x8BC1: "units",
	0x8BC2: "type",
	0x8BC3: "hidden",
}
var htmlATTRS36 = map[int]string{
	0x804A: "align",
}
var htmlATTRS37 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938B: "color",
	0x939B: "face",
	0x939C: "size",
}
var htmlATTRS38 = map[int]string{
	0x03EA: "action",
	0x03EC: "enctype",
	0x03ED: "method",
	0x03EF: "target",
	0x03F4: "accept-charset",
	0x8001: "name",
	0x977C: "onsubmit",
	0x977D: "onreset",
}
var htmlATTRS39 = map[int]string{
	0x8000: "align",
	0x8001: "name",
	0x8BB9: "src",
	0x8BBB: "border",
	0x8BBC: "frameborder",
	0x8BBD: "framespacing",
	0x8BBE: "marginwidth",
	0x8BBF: "marginheight",
	0x8BC0: "noresize",
	0x8BC1: "scrolling",
	0x8FA2: "bordercolor",
}
var htmlATTRS40 = map[int]string{
	0x03E9: "rows",
	0x03EA: "cols",
	0x03EB: "border",
	0x03EC: "bordercolor",
	0x03ED: "frameborder",
	0x03EE: "framespacing",
	0x8001: "name",
	0x9791: "onload",
	0x9792: "onunload",
	0x9798: "onbeforeunload",
	0x97B3: "onbeforeprint",
	0x97B4: "onafterprint",
}
var htmlATTRS42 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS43 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS44 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS45 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS46 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS47 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS49 = map[int]string{
	0x03EA: "noshade",
	0x8006: "width",
	0x8007: "size",
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938B: "color",
}
var htmlATTRS51 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS52 = map[int]string{
	0x8001: "name",
	0x8006: "width",
	0x8007: "height",
	0x804A: "align",
	0x8BB9: "src",
	0x8BBB: "border",
	0x8BBC: "frameborder",
	0x8BBD: "framespacing",
	0x8BBE: "marginwidth",
	0x8BBF: "marginheight",
	0x8BC0: "noresize",
	0x8BC1: "scrolling",
	0x8FA2: "vspace",
	0x8FA3: "hspace",
}
var htmlATTRS53 = map[int]string{
	0x03EB: "alt",
	0x03EC: "src",
	0x03ED: "border",
	0x03EE: "vspace",
	0x03EF: "hspace",
	0x03F0: "lowsrc",
	0x03F1: "vrml",
	0x03F2: "dynsrc",
	0x03F4: "loop",
	0x03F6: "start",
	0x07D3: "ismap",
	0x07D9: "usemap",
	0x8001: "name",
	0x8006: "width",
	0x8007: "height",
	0x8046: "title",
	0x804A: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x978D: "onabort",
	0x978E: "onerror",
	0x9791: "onload",
}
var htmlATTRS54 = map[int]string{
	0x07D1: "type",
	0x07D3: "size",
	0x07D4: "maxlength",
	0x07D6: "readonly",
	0x07D8: "indeterminate",
	0x07DA: "checked",
	0x07DB: "alt",
	0x07DC: "src",
	0x07DD: "border",
	0x07DE: "vspace",
	0x07DF: "hspace",
	0x07E0: "lowsrc",
	0x07E1: "vrml",
	0x07E2: "dynsrc",
	0x07E4: "loop",
	0x07E5: "start",
	0x8001: "name",
	0x8006: "width",
	0x8007: "height",
	0x804A: "align",
	0x93EE: "value",
	0x977B: "onselect",
	0x978D: "onabort",
	0x978E: "onerror",
	0x978F: "onchange",
	0x9791: "onload",
}
var htmlATTRS56 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS57 = map[int]string{
	0x03E9: "for",
}
var htmlATTRS58 = map[int]string{
	0x804A: "align",
}
var htmlATTRS59 = map[int]string{
	0x03EA: "value",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x939A: "type",
}
var htmlATTRS60 = map[int]string{
	0x03EE: "href",
	0x03EF: "rel",
	0x03F0: "rev",
	0x03F1: "type",
	0x03F9: "media",
	0x03FA: "target",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x978E: "onerror",
	0x9791: "onload",
}
var htmlATTRS61 = map[int]string{
	0x9399: "clear",
}
var htmlATTRS62 = map[int]string{
	0x8001: "name",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS63 = map[int]string{
	0x1771: "scrolldelay",
	0x1772: "direction",
	0x1773: "behavior",
	0x1774: "scrollamount",
	0x1775: "loop",
	0x1776: "vspace",
	0x1777: "hspace",
	0x1778: "truespeed",
	0x8006: "width",
	0x8007: "height",
	0x9785: "onbounce",
	0x978B: "onfinish",
	0x978C: "onstart",
	0xFE0C: "bgcolor",
}
var htmlATTRS65 = map[int]string{
	0x03EA: "http-equiv",
	0x03EB: "content",
	0x03EC: "url",
	0x03F6: "charset",
	0x8001: "name",
}
var htmlATTRS66 = map[int]string{
	0x03F5: "n",
}
var htmlATTRS71 = map[int]string{
	0x8000: "usemap",
	0x8001: "name",
	0x8006: "width",
	0x8007: "height",
	0x8046: "title",
	0x804A: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x8BBB: "classid",
	0x8BBC: "data",
	0x8BBF: "codebase",
	0x8BC0: "codetype",
	0x8BC1: "code",
	0x8BC2: "type",
	0x8BC5: "vspace",
	0x8BC6: "hspace",
	0x978E: "onerror",
}
var htmlATTRS72 = map[int]string{
	0x03EB: "compact",
	0x03EC: "start",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x939A: "type",
}
var htmlATTRS73 = map[int]string{
	0x03EA: "selected",
	0x03EB: "value",
}
var htmlATTRS74 = map[int]string{
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS75 = map[int]string{
	0x8000: "type",
}
var htmlATTRS76 = map[int]string{
	0x9399: "clear",
}
var htmlATTRS77 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x9399: "clear",
}
var htmlATTRS78 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS82 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS83 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS84 = map[int]string{
	0x03EA: "src",
	0x03ED: "for",
	0x03EE: "event",
	0x03F0: "defer",
	0x03F2: "type",
	0x978E: "onerror",
}
var htmlATTRS85 = map[int]string{
	0x03EB: "size",
	0x03EC: "multiple",
	0x8000: "align",
	0x8001: "name",
	0x978F: "onchange",
}
var htmlATTRS86 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS87 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS88 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS89 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS90 = map[int]string{
	0x03EB: "type",
	0x03EF: "media",
	0x8046: "title",
	0x978E: "onerror",
	0x9791: "onload",
}
var htmlATTRS91 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS92 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS93 = map[int]string{
	0x03EA: "cols",
	0x03EB: "border",
	0x03EC: "rules",
	0x03ED: "frame",
	0x03EE: "cellspacing",
	0x03EF: "cellpadding",
	0x03FA: "datapagesize",
	0x8006: "width",
	0x8007: "height",
	0x8046: "title",
	0x804A: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938A: "background",
	0x93A5: "bordercolor",
	0x93A6: "bordercolorlight",
	0x93A7: "bordercolordark",
	0xFE0C: "bgcolor",
}
var htmlATTRS94 = map[int]string{
	0x8049: "align",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS95 = map[int]string{
	0x8049: "align",
	0x93A8: "valign",
}
var htmlATTRS96 = map[int]string{
	0x07D2: "rowspan",
	0x07D3: "colspan",
	0x8006: "width",
	0x8007: "height",
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938A: "background",
	0x938E: "nowrap",
	0x93A5: "bordercolor",
	0x93A6: "bordercolorlight",
	0x93A7: "bordercolordark",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS97 = map[int]string{
	0x1B5A: "rows",
	0x1B5B: "cols",
	0x1B5C: "wrap",
	0x1B5D: "readonly",
	0x8001: "name",
	0x977B: "onselect",
	0x978F: "onchange",
}
var htmlATTRS98 = map[int]string{
	0x8049: "align",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS99 = map[int]string{
	0x07D2: "rowspan",
	0x07D3: "colspan",
	0x8006: "width",
	0x8007: "height",
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x938A: "background",
	0x938E: "nowrap",
	0x93A5: "bordercolor",
	0x93A6: "bordercolorlight",
	0x93A7: "bordercolordark",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS100 = map[int]string{
	0x8049: "align",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS102 = map[int]string{
	0x8007: "height",
	0x8046: "title",
	0x8049: "align",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x93A5: "bordercolor",
	0x93A6: "bordercolorlight",
	0x93A7: "bordercolordark",
	0x93A8: "valign",
	0xFE0C: "bgcolor",
}
var htmlATTRS103 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS104 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}
var htmlATTRS105 = map[int]string{
	0x03EB: "compact",
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
	0x939A: "type",
}
var htmlATTRS106 = map[int]string{
	0x8046: "title",
	0x804B: "style",
	0x83EA: "class",
	0x83EB: "id",
}

var htmlTagAttrs = []map[int]string{
	nil,          // 0 None
	nil,          // 1 None
	nil,          // 2 None
	htmlATTRS3,   // 3 a
	nil,          // 4 acronym
	htmlATTRS5,   // 5 address
	htmlATTRS6,   // 6 applet
	htmlATTRS7,   // 7 area
	htmlATTRS8,   // 8 b
	htmlATTRS9,   // 9 base
	htmlATTRS10,  // 10 basefont
	nil,          // 11 bdo
	htmlATTRS12,  // 12 bgsound
	htmlATTRS13,  // 13 big
	nil,          // 14 blink
	htmlATTRS15,  // 15 blockquote
	htmlATTRS16,  // 16 body
	htmlATTRS17,  // 17 br
	htmlATTRS18,  // 18 button
	htmlATTRS19,  // 19 caption
	htmlATTRS20,  // 20 center
	htmlATTRS21,  // 21 cite
	htmlATTRS22,  // 22 code
	htmlATTRS23,  // 23 col
	htmlATTRS24,  // 24 colgroup
	nil,          // 25 None
	nil,          // 26 None
	htmlATTRS27,  // 27 dd
	nil,          // 28 del
	htmlATTRS29,  // 29 dfn
	nil,          // 30 dir
	htmlATTRS31,  // 31 div
	htmlATTRS32,  // 32 dl
	htmlATTRS33,  // 33 dt
	htmlATTRS34,  // 34 em
	htmlATTRS35,  // 35 embed
	htmlATTRS36,  // 36 fieldset
	htmlATTRS37,  // 37 font
	htmlATTRS38,  // 38 form
	htmlATTRS39,  // 39 frame
	htmlATTRS40,  // 40 frameset
	nil,          // 41 None
	htmlATTRS42,  // 42 h1
	htmlATTRS43,  // 43 h2
	htmlATTRS44,  // 44 h3
	htmlATTRS45,  // 45 h4
	htmlATTRS46,  // 46 h5
	htmlATTRS47,  // 47 h6
	nil,          // 48 head
	htmlATTRS49,  // 49 hr
	nil,          // 50 html
	htmlATTRS51,  // 51 i
	htmlATTRS52,  // 52 iframe
	htmlATTRS53,  // 53 img
	htmlATTRS54,  // 54 input
	nil,          // 55 ins
	htmlATTRS56,  // 56 kbd
	htmlATTRS57,  // 57 label
	htmlATTRS58,  // 58 legend
	htmlATTRS59,  // 59 li
	htmlATTRS60,  // 60 link
	htmlATTRS61,  // 61 tag61
	htmlATTRS62,  // 62 map
	htmlATTRS63,  // 63 tag63
	nil,          // 64 tag64
	htmlATTRS65,  // 65 meta
	htmlATTRS66,  // 66 nextid
	nil,          // 67 nobr
	nil,          // 68 noembed
	nil,          // 69 noframes
	nil,          // 70 noscript
	htmlATTRS71,  // 71 object
	htmlATTRS72,  // 72 ol
	htmlATTRS73,  // 73 option
	htmlATTRS74,  // 74 p
	htmlATTRS75,  // 75 param
	htmlATTRS76,  // 76 plaintext
	htmlATTRS77,  // 77 pre
	htmlATTRS78,  // 78 q
	nil,          // 79 rp
	nil,          // 80 rt
	nil,          // 81 ruby
	htmlATTRS82,  // 82 s
	htmlATTRS83,  // 83 samp
	htmlATTRS84,  // 84 script
	htmlATTRS85,  // 85 select
	htmlATTRS86,  // 86 small
	htmlATTRS87,  // 87 span
	htmlATTRS88,  // 88 strike
	htmlATTRS89,  // 89 strong
	htmlATTRS90,  // 90 style
	htmlATTRS91,  // 91 sub
	htmlATTRS92,  // 92 sup
	htmlATTRS93,  // 93 table
	htmlATTRS94,  // 94 tbody
	htmlATTRS95,  // 95 tc
	htmlATTRS96,  // 96 td
	htmlATTRS97,  // 97 textarea
	htmlATTRS98,  // 98 tfoot
	htmlATTRS99,  // 99 th
	htmlATTRS100, // 100 thead
	nil,          // 101 title
	htmlATTRS102, // 102 tr
	htmlATTRS103, // 103 tt
	htmlATTRS104, // 104 u
	htmlATTRS105, // 105 ul
	htmlATTRS106, // 106 var
	nil,          // 107 wbr
	nil,          // 108 None
}
