package parser

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
	"github.com/markoczy/xtools/common/helpers"
	"github.com/markoczy/xtools/common/logger"
	"github.com/markoczy/xtools/xparse/def"
)

type xmlParser struct {
	log logger.Logger
}

func NewXml(log logger.Logger) def.Parser {
	return &xmlParser{log}
}

func (p *xmlParser) Parse(input string, cfg def.Config) (ret interface{}, err error) {
	var doc *xmlquery.Node
	var expr *xpath.Expr
	p.log.Debug("Input: %s", input)
	if doc, err = xmlquery.Parse(strings.NewReader(input)); err != nil {
		p.log.Error("Failed to parse XML Input Data")
		return
	}
	if expr, err = xpath.Compile(cfg.Path); err != nil {
		p.log.Error("Failed to compile XPath Query")
		return
	}

	x := expr.Evaluate(xmlquery.CreateXPathNavigator(doc))
	switch v := x.(type) {
	case bool, float64, string:
		ret = v
	case *xpath.NodeIterator:
		p.log.Info("Found nodeIterator, decoding")
		ret = p.decode(v, cfg)
	default:
		p.log.Error("Unhandled node type: %v", v)
		err = fmt.Errorf("Unhandled node type: %v", v)
	}
	return
}

func (p *xmlParser) decode(it *xpath.NodeIterator, cfg def.Config) interface{} {
	ret := []interface{}{}
	for it.MoveNext() {
		_, val := p.parseNode(it.Current(), cfg)
		p.log.Debug("xmlDecode Val is: %v", val)
		if val != nil {
			ret = append(ret, val)
		}
	}
	switch len(ret) {
	case 0:
		return nil
	case 1:
		return ret[0]
	default:
		return ret
	}
}

func (p *xmlParser) parseNode(node xpath.NodeNavigator, cfg def.Config) (string, interface{}) {
	p.log.Debug("parseXmlNode, type = %v", node.NodeType())
	switch node.NodeType() {
	case xpath.TextNode:
		p.log.Debug("parseXmlNode found TextNode, value: %s", node.Value())
		if helpers.Normalize(node.Value()) == "" {
			// ignore text between nodes
			return "", nil
		}
		return cfg.XmlTextField, node.Value()
	case xpath.RootNode:
		// TODO root node should not be named
		p.log.Debug("parseXmlNode found RootNode")
		name := "@root"
		k := []interface{}{}
		k = append(k, p.parseElementNode(node, cfg))
		for node.MoveToNext() {
			k = append(k, p.parseElementNode(node, cfg))
		}
		if len(k) == 1 {
			return name, k[0]
		}
		return name, k
	case xpath.ElementNode:
		name := node.LocalName()
		p.log.Debug("parseXmlNode found ElementNode, name: %s", name)
		k := []map[string]interface{}{}
		k = append(k, p.parseElementNode(node, cfg))
		p.log.Debug("parseXmlNode ElementNode value %v", k[0])
		return cfg.XmlChildPrefix + name, k[0]
	case xpath.AttributeNode:
		p.log.Debug("Attribute node Name: %s, Value: %s", node.LocalName(), node.Value())
		return node.LocalName(), node.Value()
	default:
		return "", nil
	}
}

func (p *xmlParser) parseElementNode(node xpath.NodeNavigator, cfg def.Config) map[string]interface{} {
	ret := map[string]interface{}{}
	for node.MoveToNextAttribute() {
		p.log.Debug("parseXmlElementNode attribute %s %s", node.LocalName(), node.Value())
		ret[cfg.XmlAttrPrefix+node.LocalName()] = node.Value()
	}
	if node.NodeType() == xpath.AttributeNode {
		node.MoveToParent()
	}
	if node.MoveToChild() {
		name, cur := p.parseNode(node, cfg)
		p.log.Debug("parseXmlElementNode child %s %v", name, cur)
		if cur != nil {
			ret[name] = cur
		}
		for node.MoveToNext() {
			p.log.Debug("parseXmlElementNode found next")
			name, cur = p.parseNode(node, cfg)
			if cur != nil {
				defined := ret[name]
				switch {
				case defined == nil:
					p.log.Debug("* parseXmlElementNode Found nil")
					ret[name] = cur
				case reflect.ValueOf(defined).Kind() == reflect.Slice:
					p.log.Debug("* parseXmlElementNode Found array")
					ret[name] = append(defined.([]interface{}), cur)
				default:
					p.log.Debug("* parseXmlElementNode Found other %v", reflect.ValueOf(defined).Kind())
					arr := []interface{}{}
					arr = append(arr, defined, cur)
					ret[name] = arr
				}
			}
		}
		node.MoveToParent()
	}
	return ret
}
