# 上游逐项覆盖清单

来源为当前仓库的contracts.json；命名契约数包含不同分组对相同URL的独立定义。候选接口默认关闭。每项对应生成的Go方法并由HTTP契约测试逐项调用。

## 千寻：99项

| 模式 | Go方法 | 上游操作 | 官方来源 |
|---|---|---|---|
| framework | GetFriendList | getFriendList | [官方文档](https://qxpro1.apifox.cn/api-177838774) |
| framework | DownloadFileTo | downloadFile | [官方文档](https://qxpro1.apifox.cn/api-177838775) |
| framework | GetGroupList | getGroupList | [官方文档](https://qxpro1.apifox.cn/api-177838776) |
| framework | GetPublicList | getPublicList | [官方文档](https://qxpro1.apifox.cn/api-177838777) |
| framework | CheckWeChat | checkWeChat | [官方文档](https://qxpro1.apifox.cn/api-177838778) |
| framework | SendText | sendText | [官方文档](https://qxpro1.apifox.cn/api-177838780) |
| framework | SendImage | sendImage | [官方文档](https://qxpro1.apifox.cn/api-177838781) |
| framework | SendFile | sendFile | [官方文档](https://qxpro1.apifox.cn/api-177838782) |
| framework | SendGif | sendGif | [官方文档](https://qxpro1.apifox.cn/api-177838783) |
| framework | QueryObj | queryObj | [官方文档](https://qxpro1.apifox.cn/api-177838784) |
| framework | QueryGroup | queryGroup | [官方文档](https://qxpro1.apifox.cn/api-177838785) |
| framework | DecryptImage | decryptImage | [官方文档](https://qxpro1.apifox.cn/api-177838787) |
| framework | GetSelfInfo | getSelfInfo | [官方文档](https://qxpro1.apifox.cn/api-177838791) |
| framework | GetMemberNick | getMemberNick | [官方文档](https://qxpro1.apifox.cn/api-177838792) |
| framework | EditObjRemark | editObjRemark | [官方文档](https://qxpro1.apifox.cn/api-177838793) |
| framework | EditSelfMemberNick | editSelfMemberNick | [官方文档](https://qxpro1.apifox.cn/api-177838794) |
| framework | DelFriend | delFriend | [官方文档](https://qxpro1.apifox.cn/api-177838795) |
| framework | ConfirmTrans | confirmTrans | [官方文档](https://qxpro1.apifox.cn/api-177838796) |
| framework | QuitGroup | quitGroup | [官方文档](https://qxpro1.apifox.cn/api-177838797) |
| framework | CreateGroup | createGroup | [官方文档](https://qxpro1.apifox.cn/api-177838798) |
| framework | GetMemberList | getMemberList | [官方文档](https://qxpro1.apifox.cn/api-177838799) |
| framework | SendShareURL | sendShareUrl | [官方文档](https://qxpro1.apifox.cn/api-177838800) |
| framework | SendApplet | sendApplet | [官方文档](https://qxpro1.apifox.cn/api-177838801) |
| framework | SendMusic | sendMusic | [官方文档](https://qxpro1.apifox.cn/api-177838802) |
| framework | SendChatLog | sendChatLog | [官方文档](https://qxpro1.apifox.cn/api-177838803) |
| framework | SendXML | sendXml | [官方文档](https://qxpro1.apifox.cn/api-177838804) |
| framework | AddMembers | addMembers | [官方文档](https://qxpro1.apifox.cn/api-177838805) |
| framework | InviteMembers | inviteMembers | [官方文档](https://qxpro1.apifox.cn/api-177838806) |
| framework | DelMembers | delMembers | [官方文档](https://qxpro1.apifox.cn/api-177838807) |
| framework | AgreeFriendReq | agreeFriendReq | [官方文档](https://qxpro1.apifox.cn/api-177838808) |
| framework | GetWeChatList | getWeChatList | [官方文档](https://qxpro1.apifox.cn/api-177839467) |
| framework | OpenBrowser | openBrowser | [官方文档](https://qxpro1.apifox.cn/api-179610715) |
| framework | SendText2 | sendText2 | [官方文档](https://qxpro1.apifox.cn/api-179619383) |
| framework | SendReferText | sendReferText | [官方文档](https://qxpro1.apifox.cn/api-179619384) |
| framework | AddFriendByV3 | addFriendByV3 | [官方文档](https://qxpro1.apifox.cn/api-179619385) |
| framework | QueryNewFriend | queryNewFriend | [官方文档](https://qxpro1.apifox.cn/api-179619386) |
| framework | ReturnTrans | returnTrans | [官方文档](https://qxpro1.apifox.cn/api-186718983) |
| framework | SendCard | sendCard | [官方文档](https://qxpro1.apifox.cn/api-186719200) |
| framework | AddFriendByGroupWXID | addFriendByGroupWxid | [官方文档](https://qxpro1.apifox.cn/api-196052494) |
| framework | GetLoginQRCode | getLoginQrCode | [官方文档](https://qxpro1.apifox.cn/api-205377395) |
| framework | GetAuthInfo | getAuthInfo | [官方文档](https://qxpro1.apifox.cn/api-262423735) |
| framework | GetLabelList | getLabelList | [官方文档](https://qxpro1.apifox.cn/api-267252174) |
| framework | RunCloudFunction | runCloudFunction | [官方文档](https://qxpro1.apifox.cn/api-272012484) |
| framework | SetReadStatus | setReadStatus | [官方文档](https://qxpro1.apifox.cn/api-327315717) |
| framework | KillWeChat | killWeChat | [官方文档](https://qxpro1.apifox.cn/api-346683064) |
| framework | RunWeChat | runWeChat | [官方文档](https://qxpro1.apifox.cn/api-346683073) |
| framework | SendLocationInfo | sendLocationInfo | [官方文档](https://qxpro1.apifox.cn/api-346686404) |
| framework | AuthCami | authCami | [官方文档](https://qxpro1.apifox.cn/api-358048621) |
| framework | RevokeMyMsg | revokeMyMsg | [官方文档](https://qxpro1.apifox.cn/api-358122605) |
| direct | GetFriendList | getFriendList | [官方文档](https://qxpro1.apifox.cn/api-177736858) |
| direct | DownloadFileTo | downloadFile | [官方文档](https://qxpro1.apifox.cn/api-177736859) |
| direct | GetGroupList | getGroupList | [官方文档](https://qxpro1.apifox.cn/api-177736860) |
| direct | GetPublicList | getPublicList | [官方文档](https://qxpro1.apifox.cn/api-177736861) |
| direct | CheckWeChat | checkWeChat | [官方文档](https://qxpro1.apifox.cn/api-177736862) |
| direct | GetLoginStatus | getLoginStatus | [官方文档](https://qxpro1.apifox.cn/api-177736863) |
| direct | SendText | sendText | [官方文档](https://qxpro1.apifox.cn/api-177736864) |
| direct | SendImage | sendImage | [官方文档](https://qxpro1.apifox.cn/api-177736865) |
| direct | SendFile | sendFile | [官方文档](https://qxpro1.apifox.cn/api-177736866) |
| direct | SendGif | sendGif | [官方文档](https://qxpro1.apifox.cn/api-177736867) |
| direct | QueryObj | queryObj | [官方文档](https://qxpro1.apifox.cn/api-177736868) |
| direct | GetSelfInfo | getSelfInfo | [官方文档](https://qxpro1.apifox.cn/api-177736869) |
| direct | GetMemberNick | getMemberNick | [官方文档](https://qxpro1.apifox.cn/api-177736870) |
| direct | EditObjRemark | editObjRemark | [官方文档](https://qxpro1.apifox.cn/api-177736871) |
| direct | DelFriend | delFriend | [官方文档](https://qxpro1.apifox.cn/api-177736872) |
| direct | ConfirmTrans | confirmTrans | [官方文档](https://qxpro1.apifox.cn/api-177736873) |
| direct | QuitGroup | quitGroup | [官方文档](https://qxpro1.apifox.cn/api-177736874) |
| direct | GetMemberList | getMemberList | [官方文档](https://qxpro1.apifox.cn/api-177736875) |
| direct | SendShareURL | sendShareUrl | [官方文档](https://qxpro1.apifox.cn/api-177736876) |
| direct | SendApplet | sendApplet | [官方文档](https://qxpro1.apifox.cn/api-177736877) |
| direct | SendMusic | sendMusic | [官方文档](https://qxpro1.apifox.cn/api-177736878) |
| direct | SendChatLog | sendChatLog | [官方文档](https://qxpro1.apifox.cn/api-177736879) |
| direct | SendXML | sendXml | [官方文档](https://qxpro1.apifox.cn/api-177736880) |
| direct | AddMembers | addMembers | [官方文档](https://qxpro1.apifox.cn/api-177785208) |
| direct | InviteMembers | inviteMembers | [官方文档](https://qxpro1.apifox.cn/api-177785340) |
| direct | DelMembers | delMembers | [官方文档](https://qxpro1.apifox.cn/api-177785365) |
| direct | AgreeFriendReq | agreeFriendReq | [官方文档](https://qxpro1.apifox.cn/api-177785406) |
| direct | EditSelfMemberNick | editSelfMemberNick | [官方文档](https://qxpro1.apifox.cn/api-177785472) |
| direct | CreateGroup | createGroup | [官方文档](https://qxpro1.apifox.cn/api-177785586) |
| direct | QueryGroup | queryGroup | [官方文档](https://qxpro1.apifox.cn/api-177785912) |
| direct | SetDownloadImage | setDownloadImage | [官方文档](https://qxpro1.apifox.cn/api-177786909) |
| direct | KillWeChat | killWeChat | [官方文档](https://qxpro1.apifox.cn/api-177786967) |
| direct | GetLoginQRCode | getLoginQrCode | [官方文档](https://qxpro1.apifox.cn/api-177787147) |
| direct | DecryptImage | decryptImage | [官方文档](https://qxpro1.apifox.cn/api-177787214) |
| direct | EditVersion | editVersion | [官方文档](https://qxpro1.apifox.cn/api-177800324) |
| direct | SendText2 | sendText2 | [官方文档](https://qxpro1.apifox.cn/api-179605533) |
| direct | SendReferText | sendReferText | [官方文档](https://qxpro1.apifox.cn/api-179606577) |
| direct | AddFriendByV3 | addFriendByV3 | [官方文档](https://qxpro1.apifox.cn/api-179610583) |
| direct | QueryNewFriend | queryNewFriend | [官方文档](https://qxpro1.apifox.cn/api-179613076) |
| direct | OpenBrowser | openBrowser | [官方文档](https://qxpro1.apifox.cn/api-179619387) |
| direct | ReturnTrans | returnTrans | [官方文档](https://qxpro1.apifox.cn/api-186719042) |
| direct | SendCard | sendCard | [官方文档](https://qxpro1.apifox.cn/api-186825692) |
| direct | AddFriendByGroupWXID | addFriendByGroupWxid | [官方文档](https://qxpro1.apifox.cn/api-194636762) |
| direct | GetAuthInfo | getAuthInfo | [官方文档](https://qxpro1.apifox.cn/api-262420985) |
| direct | GetLabelList | getLabelList | [官方文档](https://qxpro1.apifox.cn/api-267252241) |
| direct | RunCloudFunction | runCloudFunction | [官方文档](https://qxpro1.apifox.cn/api-272618248) |
| direct | SetReadStatus | setReadStatus | [官方文档](https://qxpro1.apifox.cn/api-327316196) |
| direct | SendLocationInfo | sendLocationInfo | [官方文档](https://qxpro1.apifox.cn/api-346682175) |
| direct | AuthCami | authCami | [官方文档](https://qxpro1.apifox.cn/api-358048794) |
| direct | RevokeMyMsg | revokeMyMsg | [官方文档](https://qxpro1.apifox.cn/api-358122719) |

## 米哈游及相关上游：179项

| Go分组.方法 | HTTP | 来源规范 | 候选 | 签名 |
|---|---|---|---|---|
| AccountVerifier.CreateEmailCaptchaByActionTicket | POST https://sg-public-api.hoyoverse.com/account/ma-verifier/api/createEmailCaptchaByActionTicket | docs/contracts/mihoyo/account-verifier.openapi.json | False | none |
| AccountVerifier.GetActionTicketInfo | POST https://sg-public-api.hoyoverse.com/account/ma-verifier/api/getActionTicketInfo | docs/contracts/mihoyo/account-verifier.openapi.json | False | none |
| AccountVerifier.VerifyActionTicketPartly | POST https://sg-public-api.hoyoverse.com/account/ma-verifier/api/verifyActionTicketPartly | docs/contracts/mihoyo/account-verifier.openapi.json | False | none |
| Enka.GetStarRailShowcase | GET https://enka.network/api/hsr/uid/{uid} | docs/contracts/mihoyo/enka.openapi.json | False | none |
| Enka.GetGenshinShowcase | GET https://enka.network/api/uid/{uid} | docs/contracts/mihoyo/enka.openapi.json | False | none |
| Enka.GetZZZShowcase | GET https://enka.network/api/zzz/uid/{uid} | docs/contracts/mihoyo/enka.openapi.json | False | none |
| GachaAuth.GenAuthKey | POST https://api-os-takumi.hoyoverse.com/binding/api/genAuthKey | docs/contracts/mihoyo/gacha-auth.openapi.json | False | legacy-ds2 |
| GenshinBBSGameRecord.GetGenshinBBSActivityCalendar | POST https://bbs-api-os.hoyolab.com/game_record/app/genshin/api/act_calendar | docs/contracts/mihoyo/genshin-bbs-game-record.openapi.json | False | app-ds2 |
| GenshinBBSGameRecord.GetGenshinBBSCharacterDetail | POST https://bbs-api-os.hoyolab.com/game_record/app/genshin/api/character/detail | docs/contracts/mihoyo/genshin-bbs-game-record.openapi.json | False | app-ds2 |
| GenshinBBSGameRecord.GetGenshinBBSCharacterList | POST https://bbs-api-os.hoyolab.com/game_record/app/genshin/api/character/list | docs/contracts/mihoyo/genshin-bbs-game-record.openapi.json | False | app-ds2 |
| GenshinBBSGameRecord.GetGenshinBBSDailyNote | GET https://bbs-api-os.hoyolab.com/game_record/app/genshin/api/dailyNote | docs/contracts/mihoyo/genshin-bbs-game-record.openapi.json | False | app-ds2 |
| GenshinBBSGameRecord.GetGenshinBBSIndex | GET https://bbs-api-os.hoyolab.com/game_record/app/genshin/api/index | docs/contracts/mihoyo/genshin-bbs-game-record.openapi.json | False | app-ds2 |
| GenshinBBSGameRecord.GetGenshinBBSSpiralAbyss | GET https://bbs-api-os.hoyolab.com/game_record/app/genshin/api/spiralAbyss | docs/contracts/mihoyo/genshin-bbs-game-record.openapi.json | False | app-ds2 |
| GenshinGacha.GetGenshinGachaLog | GET https://public-operation-hk4e-sg.hoyoverse.com/gacha_info/api/getGachaLog | docs/contracts/mihoyo/genshin-gacha.openapi.json | False | none |
| GenshinGameRecord.GetGenshinAchievements | POST https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/achievement | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinActivityCalendar | POST https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/act_calendar | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinActivities | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/activities | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinAvatarBasicInfo | POST https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/avatarBasicInfo | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinCharacterMastery | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/char_master | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.CancelGenshinMasteryRedDot | POST https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/char_master/red_dot/cancel | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinCharacterDetails | POST https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/character/detail | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinCharacterList | POST https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/character/list | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinDailyNote | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/dailyNote | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinExternalContent | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/external_content | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinGcgBasicInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/gcg/basicInfo | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinGcgCardBackList | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/gcg/cardBackList | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinGcgCardList | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/gcg/cardList | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinGcgChallengeDeck | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/gcg/challenge/deck | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinGcgChallengeSchedule | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/gcg/challenge/schedule | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinHardChallenge | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/hard_challenge | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinHardChallengePopularity | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/hard_challenge/popularity | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinIndex | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/index | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinRoleBasicInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/roleBasicInfo | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinRoleCombat | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/role_combat | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinSpiralAbyss | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/spiralAbyss | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.GetGenshinTools | GET https://sg-act-public-api.hoyolab.com/event/game_record/genshin/api/tool_oversea | docs/contracts/mihoyo/genshin-game-record.openapi.json | False | none |
| GenshinGameRecord.SetGenshinRecommendProperty | POST https://sg-act-public-api.hoyolab.com/genshin/api/character/set_recommend_prop | docs/contracts/mihoyo/genshin-game-record.openapi.json | True | none |
| GenshinGameRecord.SetGenshinCharacterTop | POST https://sg-act-public-api.hoyolab.com/genshin/api/character/top | docs/contracts/mihoyo/genshin-game-record.openapi.json | True | none |
| GenshinGameRecord.SetGenshinGcgPrivacy | POST https://sg-act-public-api.hoyolab.com/genshin/api/gcg/setPrivacySettings | docs/contracts/mihoyo/genshin-game-record.openapi.json | True | none |
| GenshinOverseasEvents.GetGenshinOverseasHardChallenge | GET https://sg-public-api.hoyolab.com/event/game_record/genshin/api/hard_challenge | docs/contracts/mihoyo/genshin-overseas-events.openapi.json | False | none |
| GenshinOverseasEvents.GetGenshinOverseasRoleCombat | GET https://sg-public-api.hoyolab.com/event/game_record/genshin/api/role_combat | docs/contracts/mihoyo/genshin-overseas-events.openapi.json | False | none |
| GenshinRedeem.RedeemCode | GET https://sg-hk4e-api.hoyolab.com/common/apicdkey/api/webExchangeCdkey | docs/contracts/mihoyo/genshin-redeem.openapi.json | False | none |
| GenshinSign.SignIn | POST https://sg-hk4e-api.hoyolab.com/event/sol/sign | docs/contracts/mihoyo/genshin-sign.openapi.json | False | none |
| HoyolabGameCard.GetHoyolabGameRecordCardFromActAPI | GET https://sg-act-public-api.hoyolab.com/game_record/card/wapi/getGameRecordCard | docs/contracts/mihoyo/hoyolab-game-card.openapi.json | False | none |
| HoyoverseAppAuth.AppLoginByPassword | POST https://sg-public-api.hoyoverse.com/account/ma-passport/api/appLoginByPassword | docs/contracts/mihoyo/hoyoverse-app-auth.openapi.json | False | none |
| HoyoverseGameRoles.GetUserGameRolesByCookie | GET https://api-os-takumi.hoyoverse.com/binding/api/getUserGameRolesByCookie | docs/contracts/mihoyo/hoyoverse-game-roles.openapi.json | False | none |
| HoyoverseTokenExchange.GetCookieAccountInfoBySToken | POST https://api-account-os.hoyoverse.com/account/auth/api/getCookieAccountInfoBySToken | docs/contracts/mihoyo/hoyoverse-token-exchange.openapi.json | False | none |
| HoyoverseTokenExchange.GetLTokenBySToken | POST https://api-account-os.hoyoverse.com/account/auth/api/getLTokenBySToken | docs/contracts/mihoyo/hoyoverse-token-exchange.openapi.json | False | none |
| HoyowikiPV.IncreaseHoyowikiZZZPV | POST https://sg-act-public-api.hoyolab.com/hoyowiki/zzz/wapi/entry_page/pv | docs/contracts/mihoyo/hoyowiki-pv.openapi.json | False | none |
| HoyowikiZZZ.GetHoyowikiZZZEntryPage | GET https://sg-act-public-api-static.hoyolab.com/hoyowiki/zzz/wapi/entry_page | docs/contracts/mihoyo/hoyowiki-zzz.openapi.json | False | none |
| HoyowikiZZZ.GetHoyowikiZZZHistory | GET https://sg-act-public-api-static.hoyolab.com/hoyowiki/zzz/wapi/entry_page/history | docs/contracts/mihoyo/hoyowiki-zzz.openapi.json | False | none |
| HoyowikiZZZ.GetHoyowikiZZZEntryPages | GET https://sg-act-public-api-static.hoyolab.com/hoyowiki/zzz/wapi/entry_pages | docs/contracts/mihoyo/hoyowiki-zzz.openapi.json | False | none |
| HoyowikiZZZ.GetHoyowikiEditPermission | POST https://sg-act-public-api-static.hoyolab.com/hoyowiki/zzz/wapi/get_edit_permission | docs/contracts/mihoyo/hoyowiki-zzz.openapi.json | False | none |
| HoyowikiZZZ.GetHoyowikiPersonList | GET https://sg-act-public-api-static.hoyolab.com/hoyowiki/zzz/wapi/get_person_list | docs/contracts/mihoyo/hoyowiki-zzz.openapi.json | False | none |
| MihoyoCommunity.GetCreatorCard | GET https://bbs-api-os.hoyolab.com/community/creator/wapi/user/card | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.GetAccountBriefInfo | GET https://bbs-api-os.hoyolab.com/community/misc/wapi/account/user_brief_info | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.GetUserUnreadCount | GET https://bbs-api-os.hoyolab.com/community/notification/wapi/getUserUnreadCount | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.GetFullUserProfile | GET https://bbs-api-os.hoyolab.com/community/painter/wapi/user/full | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.PostFullUserProfile | POST https://bbs-api-os.hoyolab.com/community/painter/wapi/user/full | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.GetUserPosts | GET https://bbs-api-os.hoyolab.com/community/post/wapi/userPost | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.GetUserAssets | GET https://bbs-api-os.hoyolab.com/community/user/wapi/asset/list | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.GetUserBadges | GET https://bbs-api-os.hoyolab.com/community/user/wapi/badge/list | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoCommunity.GetGameRecordCard | GET https://bbs-api-os.hoyolab.com/game_record/card/wapi/getGameRecordCard | docs/contracts/mihoyo/mihoyo-community.openapi.json | False | none |
| MihoyoDeviceFingerprint.GetDeviceFingerprintExtList | GET https://sg-public-data-api.hoyoverse.com/device-fp/api/getExtList | docs/contracts/mihoyo/mihoyo-device-fingerprint.openapi.json | False | none |
| MihoyoDeviceFingerprint.GetDeviceFingerprint | POST https://sg-public-data-api.hoyoverse.com/device-fp/api/getFp | docs/contracts/mihoyo/mihoyo-device-fingerprint.openapi.json | False | none |
| MihoyoGuides.GetStarRailGuideCatalog | GET https://act-api-takumi-static.mihoyo.com/common/blackboard/sr_wiki/v1/home/content/list | docs/contracts/mihoyo/mihoyo-guides.openapi.json | False | none |
| MihoyoGuides.GetGenshinGuideCatalog | GET https://act-api-takumi-static.mihoyo.com/common/blackboard/ys_strategy/v1/home/content/list | docs/contracts/mihoyo/mihoyo-guides.openapi.json | False | none |
| MihoyoGuides.GetZZZAgentCatalog | GET https://act-api-takumi-static.mihoyo.com/common/blackboard/zzz_wiki/v1/home/content/list | docs/contracts/mihoyo/mihoyo-guides.openapi.json | False | none |
| MihoyoGuides.GetZZZGuideEntryPage | GET https://act-api-takumi-static.mihoyo.com/hoyowiki/zzz/wapi/entry_page | docs/contracts/mihoyo/mihoyo-guides.openapi.json | False | none |
| MihoyoLoginSwitch.GetLoginSwitchStatus | GET https://sg-public-api-static.hoyolab.com/account/ma-passport/api/getSwitchStatus | docs/contracts/mihoyo/mihoyo-login-switch.openapi.json | False | none |
| MihoyoPassport.PassportLogout | POST https://passport-api-eu.hoyolab.com/account/ma-passport/api/logout | docs/contracts/mihoyo/mihoyo-passport.openapi.json | True | none |
| MihoyoPassport.WebLoginByEmailCaptcha | POST https://passport-api-eu.hoyolab.com/account/ma-passport/api/webLoginByEmailCaptcha | docs/contracts/mihoyo/mihoyo-passport.openapi.json | True | none |
| MihoyoPassport.WebLoginByPassword | POST https://passport-api-eu.hoyolab.com/account/ma-passport/api/webLoginByPassword | docs/contracts/mihoyo/mihoyo-passport.openapi.json | False | none |
| MihoyoPassport.CrossLoginStart | POST https://passport-api-eu.hoyolab.com/account/ma-passport/token/crossLoginStart | docs/contracts/mihoyo/mihoyo-passport.openapi.json | True | none |
| MihoyoPassport.CrossLogout | POST https://passport-api-eu.hoyolab.com/account/ma-passport/token/crossLogout | docs/contracts/mihoyo/mihoyo-passport.openapi.json | True | none |
| MihoyoPassport.VerifyLToken | POST https://passport-api-eu.hoyolab.com/account/ma-passport/token/verifyLToken | docs/contracts/mihoyo/mihoyo-passport.openapi.json | False | none |
| MiyousheArticles.GetMiyoushePostFull | GET https://bbs-api.miyoushe.com/post/wapi/getPostFull | docs/contracts/mihoyo/miyoushe-articles.openapi.json | False | none |
| PassportExtra.VerifyCookieToken | POST https://passport-api-eu.hoyolab.com/account/ma-passport/token/verifyCookieToken | docs/contracts/mihoyo/passport-extra.openapi.json | False | none |
| StarRailGachaCN.GetGachaLog | GET https://public-operation-hkrpg.mihoyo.com/common/gacha_record/api/getGachaLog | docs/contracts/mihoyo/star-rail-gacha-cn.openapi.json | False | none |
| StarRailGachaCN.GetLdGachaLog | GET https://public-operation-hkrpg.mihoyo.com/common/gacha_record/api/getLdGachaLog | docs/contracts/mihoyo/star-rail-gacha-cn.openapi.json | False | none |
| StarRailGachaGlobal.GetGachaLog | GET https://public-operation-hkrpg-sg.hoyoverse.com/common/gacha_record/api/getGachaLog | docs/contracts/mihoyo/star-rail-gacha-global.openapi.json | False | none |
| StarRailGachaGlobal.GetLdGachaLog | GET https://public-operation-hkrpg-sg.hoyoverse.com/common/gacha_record/api/getLdGachaLog | docs/contracts/mihoyo/star-rail-gacha-global.openapi.json | False | none |
| StarRailOfficialGacha.GetOfficialGachaSession | GET https://sg-act-public-api.hoyolab.com/common/badge/v1/login/info | docs/contracts/mihoyo/star-rail-official-gacha.openapi.json | False | none |
| StarRailOfficialGacha.GetOfficialFiveStarList | GET https://sg-act-public-api.hoyolab.com/event/rpg_gacha_record/five_star_list | docs/contracts/mihoyo/star-rail-official-gacha.openapi.json | False | none |
| StarRailRedeem.RedeemCode | GET https://sg-hkrpg-api.hoyolab.com/common/apicdkey/api/webExchangeCdkey | docs/contracts/mihoyo/star-rail-redeem.openapi.json | False | none |
| StarRailSign.SignIn | POST https://sg-act-public-api.hoyolab.com/event/luna/hkrpg/os/sign | docs/contracts/mihoyo/star-rail-sign.openapi.json | False | none |
| StarRailWeb.GetStarRailAppAchievementInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/achievement_info | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppActivity | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/activity | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppAvatarInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/avatar/info | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppChallenge | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/challenge | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppChallengeBoss | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/challenge_boss | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppChallengeBrief | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/challenge_brief | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppChallengePeak | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/challenge_peak | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppChallengeStory | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/challenge_story | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppChestInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/chest_info | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppActivityCalendar | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/get_act_calender | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppGridFight | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/grid_fight | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppIndex | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/index | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppMuseumBasic | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/museum/basic | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppMuseumDetail | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/museum/detail | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppNote | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/note | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppPeriodicActivity | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/periodic_act | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppResident | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/resident | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppRogue | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/rogue | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppRogueLocust | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/rogue_locust | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppRogueMagic | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/rogue_magic | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppRogueNous | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/rogue_nous | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppRogueTourn | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/rogue_tourn | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppRoleBasicInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/role/basicInfo | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailWeb.GetStarRailAppTool | GET https://sg-act-public-api.hoyolab.com/event/game_record/hkrpg/api/tool | docs/contracts/mihoyo/star-rail-web.openapi.json | False | none |
| StarRailApp.GetStarRailAppAchievementInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/achievement_info | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppActivity | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/activity | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppAvatarInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/avatar/info | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppChallenge | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/challenge | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppChallengeBoss | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/challenge_boss | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppChallengeBrief | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/challenge_brief | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppChallengePeak | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/challenge_peak | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppChallengeStory | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/challenge_story | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppChestInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/chest_info | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppActivityCalendar | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/get_act_calender | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppGridFight | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/grid_fight | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppIndex | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/index | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppMuseumBasic | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/museum/basic | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppMuseumDetail | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/museum/detail | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppNote | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/note | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppPeriodicActivity | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/periodic_act | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppResident | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/resident | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppRogue | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/rogue | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppRogueLocust | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/rogue_locust | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppRogueMagic | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/rogue_magic | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppRogueNous | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/rogue_nous | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppRogueTourn | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/rogue_tourn | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppRoleBasicInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/role/basicInfo | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailApp.GetStarRailAppTool | GET https://sg-act-public-api.hoyolab.com/event/game_record/app/hkrpg/api/tool | docs/contracts/mihoyo/starrail-app.openapi.json | False | app-ds2 |
| StarRailBBS.GetStarRailBBSAchievementInfo | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/achievement_info | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSActivity | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/activity | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSAvatarInfo | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/avatar/info | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSChallenge | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/challenge | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSChallengeBoss | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/challenge_boss | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSChallengeBrief | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/challenge_brief | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSChallengePeak | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/challenge_peak | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSChallengeStory | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/challenge_story | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSChestInfo | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/chest_info | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSActivityCalendar | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/get_act_calender | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSGridFight | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/grid_fight | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSIndex | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/index | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSMuseumBasic | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/museum/basic | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSMuseumDetail | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/museum/detail | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSNote | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/note | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSPeriodicActivity | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/periodic_act | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSResident | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/resident | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSRogue | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/rogue | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSRogueLocust | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/rogue_locust | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSRogueMagic | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/rogue_magic | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSRogueNous | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/rogue_nous | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSRogueTourn | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/rogue_tourn | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSRoleBasicInfo | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/role/basicInfo | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| StarRailBBS.GetStarRailBBSTool | GET https://bbs-api-os.hoyolab.com/game_record/hkrpg/api/tool | docs/contracts/mihoyo/starrail-bbs.openapi.json | False | hsr-ds1 |
| ZZZGameRecord.GetZZZAbyssAbstract | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/abyss_abstract | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZAbyssS2Abstract | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/abysss2_abstract | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZActivityCalendar | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/activity_calendar | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZAvatarBasic | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/avatar/basic | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZAvatarInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/avatar/info | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZBuddyInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/buddy/info | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZClimbingTower | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/climbing_tower_detail | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZCurrentGacha | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/cur_gacha_detail | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZExplorationDetail | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/exploration_detail | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZGachaCalendar | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/gacha_calendar | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZGachaRecord | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/gacha_record | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZHadalInfo | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/hadal_info_v2 | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZHoloBoss | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/holo_boss_detail | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZIndex | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/index | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZMemDetail | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/mem_detail | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZNote | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/note | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZVoidFrontAbstract | GET https://sg-act-public-api.hoyolab.com/event/game_record_zzz/api/zzz/void_front_battle_period_abstract_info | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZFeedbackSchedule | GET https://sg-act-public-api.hoyolab.com/event/nap_feedback/user/get_cur_schedule | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZGameRecord.GetZZZMonthlyLedger | GET https://sg-act-public-api.hoyolab.com/event/nap_ledger/month_info | docs/contracts/mihoyo/zzz-game-record.openapi.json | False | none |
| ZZZRedeem.RedeemCode | POST https://public-operation-nap.hoyoverse.com/common/apicdkey/api/webExchangeCdkeyRisk | docs/contracts/mihoyo/zzz-redeem.openapi.json | False | none |
| ZZZSign.SignIn | POST https://sg-act-public-api.hoyolab.com/event/luna/zzz/os/sign | docs/contracts/mihoyo/zzz-sign.openapi.json | False | none |
