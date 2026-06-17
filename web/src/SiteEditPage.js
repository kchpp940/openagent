// Copyright 2026 The OpenAgent Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React from "react";
import Loading from "./common/Loading";
import {Button, Card, Col, Image, Input, Modal, Popover, Row, Space, Upload} from "antd";
import {LinkOutlined, UploadOutlined} from "@ant-design/icons";
import * as SiteBackend from "./backend/SiteBackend";
import * as SystemBackend from "./backend/SystemInfo";
import * as ResourceBackend from "./backend/ResourceBackend";
import * as Setting from "./Setting";
import i18next from "i18next";
import Editor from "./common/Editor";
import {NavItemTree} from "./component/nav-item-tree/NavItemTree";
import ConfigForm from "./ConfigForm";

const EXCLUDED_CONFIG_KEYS = ["faviconUrl", "logoUrl", "navbarHtml", "footerHtml"];

class SiteEditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      classes: props,
      siteName: props.match.params.siteName,
      site: null,
      configMetadata: null,
      uploadingFavicon: false,
      uploadingLogo: false,
    };
    this.configFormRef = React.createRef();
  }

  UNSAFE_componentWillMount() {
    this.getSite();
    this.getConfigMetadata();
  }

  getConfigMetadata() {
    SystemBackend.getConfigMetadata()
      .then((res) => {
        if (res.status === "ok") {
          const filtered = (res.data || []).filter(
            (m) => !EXCLUDED_CONFIG_KEYS.includes(m.key)
          );
          this.setState({configMetadata: filtered});
        }
      })
      .catch(() => {
        this.setState({configMetadata: []});
      });
  }

  getSite() {
    SiteBackend.getSite("admin", this.state.siteName)
      .then((res) => {
        if (res.status === "ok") {
          this.setState({site: res.data});
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to get")}: ${res.msg}`);
        }
      });
  }

  updateSiteField(key, value) {
    const site = Setting.deepCopy(this.state.site);
    site[key] = value;
    this.setState({site});
  }

  handleImageUpload(field, file) {
    const loadingKey = field === "faviconUrl" ? "uploadingFavicon" : "uploadingLogo";
    this.setState({[loadingKey]: true});
    ResourceBackend.uploadResource("admin", "avatar", "site", this.state.site.name, file)
      .then((res) => {
        if (res.status === "ok") {
          this.updateSiteField(field, res.data);
          Setting.showMessage("success", i18next.t("general:Successfully uploaded"));
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to upload")}: ${res.msg}`);
        }
      })
      .catch(err => {
        Setting.showMessage("error", `${i18next.t("general:Failed to upload")}: ${err.message}`);
      })
      .finally(() => {
        this.setState({[loadingKey]: false});
      });
  }

  buildPayload() {
    const site = Setting.deepCopy(this.state.site);
    const configValues = this.configFormRef.current
      ? this.configFormRef.current.getValues()
      : {};
    return {...site, ...configValues};
  }

  submitSiteEdit(exitAfterSave) {
    const payload = this.buildPayload();
    SiteBackend.updateSite(payload.owner, this.state.siteName, payload)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          Setting.setThemeColor(payload.themeColor || Setting.getThemeColor());
          this.setState({siteName: payload.name});
          if (this.props.onUpdateSite) {
            this.props.onUpdateSite();
          }
          this.props.history.push(`/sites/${payload.name}`);
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`);
      });
  }

  renderSiteField(label, control, span = 8, style = {}) {
    return (
      <Col style={{marginTop: "12px", ...style}} span={Setting.isMobile() ? 22 : span}>
        <div style={{marginBottom: "6px", color: "var(--ant-color-text-secondary)", fontWeight: 500, lineHeight: "22px", fontSize: "13px"}}>{label}</div>
        {control}
      </Col>
    );
  }

  renderBrandingCustomFields() {
    const site = this.state.site;
    const rowGutter = [16, 8];
    return (
      <Row gutter={rowGutter}>
        {this.renderSiteField(
          Setting.getLabel(i18next.t("general:Favicon URL"), i18next.t("general:Favicon URL - Tooltip")),
          <Space direction="vertical" style={{width: "100%"}}>
            <Space.Compact style={{width: "100%"}}>
              <Input prefix={<LinkOutlined />} value={site.faviconUrl} onChange={e => {
                this.updateSiteField("faviconUrl", e.target.value);
              }} />
              <Upload name="file" accept="image/*" showUploadList={false} customRequest={({file}) => this.handleImageUpload("faviconUrl", file)}>
                <Button icon={<UploadOutlined />} loading={this.state.uploadingFavicon}>
                  {i18next.t("general:Upload")}
                </Button>
              </Upload>
            </Space.Compact>
            {site.faviconUrl ? (
              <Image src={Setting.getFaviconUrl("", site.faviconUrl)} alt={site.faviconUrl} height={90}
                preview={{mask: i18next.t("general:Preview")}}
              />
            ) : null}
          </Space>,
          12
        )}
        {this.renderSiteField(
          Setting.getLabel(i18next.t("general:Logo URL"), i18next.t("general:Logo URL - Tooltip")),
          <Space direction="vertical" style={{width: "100%"}}>
            <Space.Compact style={{width: "100%"}}>
              <Input prefix={<LinkOutlined />} value={site.logoUrl} onChange={e => {
                this.updateSiteField("logoUrl", e.target.value);
              }} />
              <Upload name="file" accept="image/*" showUploadList={false} customRequest={({file}) => this.handleImageUpload("logoUrl", file)}>
                <Button icon={<UploadOutlined />} loading={this.state.uploadingLogo}>
                  {i18next.t("general:Upload")}
                </Button>
              </Upload>
            </Space.Compact>
            {site.logoUrl ? (
              <Image src={Setting.getLogo("", site.logoUrl)} alt={site.logoUrl} height={90}
                preview={{mask: i18next.t("general:Preview")}}
              />
            ) : null}
          </Space>,
          12
        )}
      </Row>
    );
  }

  renderContentCustomFields() {
    const site = this.state.site;
    const rowGutter = [16, 8];
    return (
      <Row gutter={rowGutter}>
        {this.renderSiteField(
          Setting.getLabel(i18next.t("general:Navbar HTML"), i18next.t("general:Navbar HTML - Tooltip")),
          <Popover placement="right" content={
            <div style={{width: "900px", height: "300px"}}>
              <Editor
                value={site.navbarHtml}
                lang="html"
                fillHeight
                dark
                onChange={value => {
                  this.updateSiteField("navbarHtml", value);
                }}
              />
            </div>
          } title={i18next.t("general:Navbar HTML - Edit")} trigger="click">
            <Input value={site.navbarHtml} onChange={e => {
              this.updateSiteField("navbarHtml", e.target.value);
            }} />
          </Popover>,
          12
        )}
        {this.renderSiteField(
          Setting.getLabel(i18next.t("general:Footer HTML"), i18next.t("general:Footer HTML - Tooltip")),
          <Popover placement="right" content={
            <div style={{width: "900px", height: "300px"}}>
              <Editor
                value={site.footerHtml}
                lang="html"
                fillHeight
                dark
                onChange={value => {
                  this.updateSiteField("footerHtml", value);
                }}
              />
            </div>
          } title={i18next.t("store:Footer HTML - Edit")} trigger="click">
            <Input value={site.footerHtml} onChange={e => {
              this.updateSiteField("footerHtml", e.target.value);
            }} />
          </Popover>,
          12
        )}
      </Row>
    );
  }

  renderEntityCard() {
    const site = this.state.site;
    const rowGutter = [16, 8];
    const cardHeadStyle = {background: "transparent", borderBottom: "none", fontWeight: 600, fontSize: "15px", fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"};
    const sectionCardStyle = {
      marginBottom: "16px",
      borderRadius: "14px",
      boxShadow: "0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04)",
      padding: "18px",
    };
    const renderCardTitle = (title, desc) => (
      <div>
        <div style={{fontWeight: 600, fontSize: "15px"}}>{title}</div>
        <div style={{fontSize: "13px", color: "var(--ant-color-text-tertiary)", fontWeight: 400, marginTop: "2px"}}>{desc}</div>
      </div>
    );

    return (
      <Card size="small" title={renderCardTitle(i18next.t("general:Identity"), i18next.t("general:Site identity and navigation"))} style={sectionCardStyle} headStyle={cardHeadStyle}>
        <Row gutter={rowGutter}>
          {this.renderSiteField(
            Setting.getLabel(i18next.t("general:Name"), i18next.t("general:Name - Tooltip")),
            <Input value={site.name} disabled={site.name === "site-built-in"} onChange={e => {
              this.updateSiteField("name", e.target.value);
            }} />,
            8
          )}
          {this.renderSiteField(
            Setting.getLabel(i18next.t("general:Display name"), i18next.t("general:Display name - Tooltip")),
            <Input value={site.displayName} onChange={e => {
              this.updateSiteField("displayName", e.target.value);
            }} />,
            8
          )}
        </Row>
        <Row gutter={rowGutter} style={{marginTop: 8}}>
          {this.renderSiteField(
            Setting.getLabel(i18next.t("store:Navbar items"), i18next.t("store:Navbar items - Tooltip")),
            <NavItemTree
              disabled={!Setting.isAdminUser(this.props.account)}
              casdoorAvailable={Setting.isCasdoorAvailable()}
              checkedKeys={site.navItems ?? ["all"]}
              defaultExpandedKeys={["all"]}
              onCheck={(checked) => {
                const checkedArr = Array.isArray(checked) ? [...checked] : [...checked.checked];
                const identityKeys = ["/identity", "/users", "/casdoor-resources", "/permissions"];
                const prevChecked = site.navItems ?? ["all"];
                const newlyChecked = checkedArr.filter(k => identityKeys.includes(k) && !prevChecked.includes(k));
                if (!Setting.isCasdoorAvailable() && newlyChecked.length > 0) {
                  Modal.warning({
                    title: i18next.t("general:Identity requires Casdoor"),
                    content: i18next.t("general:Identity requires Casdoor - Tooltip"),
                  });
                  return;
                }
                if (!checkedArr.includes("/sites")) {
                  checkedArr.push("/sites");
                }
                this.updateSiteField("navItems", checkedArr);
              }}
            />,
            24
          )}
        </Row>
      </Card>
    );
  }

  renderConfigCategory(catName, title, desc, fieldsNode) {
    const cardHeadStyle = {background: "transparent", borderBottom: "none", fontWeight: 600, fontSize: "15px", fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"};
    const sectionCardStyle = {
      marginBottom: "16px",
      borderRadius: "14px",
      boxShadow: "0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04)",
      padding: "18px",
    };

    let customContent = null;
    if (catName === "Branding") {
      customContent = this.renderBrandingCustomFields();
    } else if (catName === "Content") {
      customContent = this.renderContentCustomFields();
    }

    const renderCardTitle = () => (
      <div>
        <div style={{fontWeight: 600, fontSize: "15px"}}>{title}</div>
        <div style={{fontSize: "13px", color: "var(--ant-color-text-tertiary)", fontWeight: 400, marginTop: "2px"}}>{desc}</div>
      </div>
    );

    return (
      <Card key={catName} size="small" title={renderCardTitle()} style={sectionCardStyle} headStyle={cardHeadStyle}>
        {customContent}
        {fieldsNode}
      </Card>
    );
  }

  renderSiteActions() {
    return (
      <Space wrap>
        <Button onClick={() => this.submitSiteEdit(false)}>{i18next.t("general:Save")}</Button>
        <Button type="primary" onClick={() => this.submitSiteEdit(true)}>{i18next.t("general:Save & Exit")}</Button>
      </Space>
    );
  }

  renderSite() {
    const {site, configMetadata} = this.state;
    if (!site || !configMetadata) {return null;}

    return (
      <div>
        <div style={{marginBottom: "16px", display: "flex", justifyContent: "space-between", alignItems: "center"}}>
          <span style={{fontSize: "22px", fontWeight: 600}}>{i18next.t("site:Edit Site")}</span>
          <div style={{display: "flex", gap: "8px", marginRight: "4px"}}>
            {this.renderSiteActions()}
          </div>
        </div>

        {this.renderEntityCard()}

        <ConfigForm
          ref={this.configFormRef}
          metadata={configMetadata}
          initialValues={site}
          renderCategory={(catName, title, desc, fieldsNode) =>
            this.renderConfigCategory(catName, title, desc, fieldsNode)
          }
        />
      </div>
    );
  }

  render() {
    const ready = this.state.site !== null && this.state.configMetadata !== null;
    return (
      <div style={{background: "var(--ant-color-bg-layout)", padding: "16px 20px 32px", minHeight: "100vh"}}>
        {ready ? this.renderSite() : <Loading type="page" tip={i18next.t("general:Loading")} />}
      </div>
    );
  }
}

export default SiteEditPage;
