export function parseMessageEvent(event) {
  const raw = event.data.data, group = event.event === 10008;
  return { source: group ? 'group' : 'private', content: raw.msg,
    senderWxid: group ? raw.finalFromWxid : raw.fromWxid, targetWxid: event.wxid,
    groupWxid: group ? raw.fromWxid : undefined, atWxidList: raw.atWxidList ?? [],
    msgId: String(raw.msgId), msgType: Number(raw.msgType), raw, event };
}
