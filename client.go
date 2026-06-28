package akikit

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Dial(ctx context.Context, addr string, dev *Device, opts ...grpc.DialOption) (*Client, func(), error) {
	dialOpts := append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, opts...)
	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, nil, err
	}
	c := New(ctx, NewAkiServiceClient(conn))
	s, err := c.cli.OpenSession(ctx, &OpenSessionRequest{Device: dev})
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	c.session = s.GetId()
	return c, func() { c.CloseSession(); conn.Close() }, nil
}

type Client struct {
	cli     AkiServiceClient
	ctx     context.Context
	session string
}

func New(ctx context.Context, cli AkiServiceClient) *Client {
	return &Client{cli: cli, ctx: ctx}
}

func NewWithSession(ctx context.Context, cli AkiServiceClient, session string) *Client {
	return &Client{cli: cli, ctx: ctx, session: session}
}

func (c *Client) Raw() AkiServiceClient { return c.cli }
func (c *Client) Session() string       { return c.session }

func (c *Client) OpenSession() (string, error) {
	s, err := c.cli.OpenSession(c.ctx, &OpenSessionRequest{})
	if err != nil {
		return "", err
	}
	c.session = s.GetId()
	return c.session, nil
}

func (c *Client) CloseSession() error {
	_, err := c.cli.CloseSession(c.ctx, &Session{Id: c.session})
	return err
}

func (c *Client) Anisette(dsid int64) (*AnisetteReply, error) {
	return c.cli.Anisette(c.ctx, &AnisetteRequest{Session: c.session, Dsid: dsid})
}

func (c *Client) AnisetteIsProvisioned(dsid int64) (*AnisetteStatusReply, error) {
	return c.cli.AnisetteIsProvisioned(c.ctx, &AnisetteRequest{Session: c.session, Dsid: dsid})
}

func (c *Client) AnisetteSynchronize(dsid int64, sim []byte) (*AnisetteSynchronizeReply, error) {
	return c.cli.AnisetteSynchronize(c.ctx, &AnisetteSynchronizeRequest{Session: c.session, Dsid: dsid, Sim: sim})
}

func (c *Client) SetRoutingInfo(dsid int64, rinfo uint64) ([]byte, error) {
	r, err := c.cli.SetRoutingInfo(c.ctx, &SetRoutingInfoRequest{Session: c.session, Dsid: dsid, Rinfo: rinfo})
	if err != nil {
		return nil, err
	}
	return r.GetAdi(), nil
}

func (c *Client) AnisetteProvisionStart(dsid int64, spim []byte) (*AnisetteProvisionStartReply, error) {
	return c.cli.AnisetteProvisionStart(c.ctx, &AnisetteProvisionStartRequest{Session: c.session, Dsid: dsid, Spim: spim})
}

func (c *Client) AnisetteProvisionEnd(provSession uint64, ptm, tk []byte) ([]byte, error) {
	r, err := c.cli.AnisetteProvisionEnd(c.ctx, &AnisetteProvisionEndRequest{Session: c.session, ProvSession: provSession, Ptm: ptm, Tk: tk})
	if err != nil {
		return nil, err
	}
	return r.GetAdi(), nil
}

func (c *Client) LoginCode(dsid int64) (*LoginCodeReply, error) {
	return c.cli.LoginCode(c.ctx, &LoginCodeRequest{Session: c.session, Dsid: dsid})
}

func (c *Client) SAPExchange(ctxh uint64, version int32, input []byte) (*SAPExchangeReply, error) {
	return c.cli.SAPExchange(c.ctx, &SAPExchangeRequest{Session: c.session, Ctx: ctxh, Version: version, Input: input})
}

func (c *Client) SAPSign(ctxh uint64, data []byte) (*SAPSignReply, error) {
	return c.cli.SAPSign(c.ctx, &SAPSignRequest{Session: c.session, Ctx: ctxh, Data: data})
}

func (c *Client) SAPPrimeSign(ctxh uint64, data []byte) (*SAPSignReply, error) {
	return c.cli.SAPPrimeSign(c.ctx, &SAPSignRequest{Session: c.session, Ctx: ctxh, Data: data})
}

func (c *Client) SAPVerify(ctxh uint64, sig, data []byte) error {
	_, err := c.cli.SAPVerify(c.ctx, &SAPVerifyRequest{Session: c.session, Ctx: ctxh, Signature: sig, Data: data})
	return err
}

func (c *Client) SAPPrimeVerify(ctxh uint64, sig, data []byte) error {
	_, err := c.cli.SAPPrimeVerify(c.ctx, &SAPVerifyRequest{Session: c.session, Ctx: ctxh, Signature: sig, Data: data})
	return err
}

func (c *Client) CloseSAPContext(ctxh uint64) error {
	_, err := c.cli.CloseSAPContext(c.ctx, &CtxHandle{Session: c.session, Ctx: ctxh})
	return err
}

func (c *Client) NACExchange1(cert []byte) (*NACExchange1Reply, error) {
	return c.cli.NACExchange1(c.ctx, &NACExchange1Request{Session: c.session, Cert: cert})
}

func (c *Client) NACExchange2(ctxh uint64, sessionData []byte) error {
	_, err := c.cli.NACExchange2(c.ctx, &NACExchange2Request{Session: c.session, Ctx: ctxh, SessionData: sessionData})
	return err
}

func (c *Client) NACSign(ctxh uint64, toSign []byte) (*NACSignReply, error) {
	return c.cli.NACSign(c.ctx, &NACSignRequest{Session: c.session, Ctx: ctxh, ToSign: toSign})
}

func (c *Client) CloseNACContext(ctxh uint64) error {
	_, err := c.cli.CloseNACContext(c.ctx, &CtxHandle{Session: c.session, Ctx: ctxh})
	return err
}

func (c *Client) SSVKeyBagSyncData(dsid int64, cval uint64) (*SSVKeyBagSyncDataReply, error) {
	return c.cli.SSVKeyBagSyncData(c.ctx, &SSVKeyBagSyncDataRequest{Session: c.session, Dsid: dsid, Cval: cval})
}

func (c *Client) QuickSign(input []byte) (*QuickSignReply, error) {
	return c.cli.QuickSign(c.ctx, &QuickSignRequest{Session: c.session, Input: input})
}

func (c *Client) RSASign(data []byte) (*RSASignReply, error) {
	return c.cli.RSASign(c.ctx, &RSASignRequest{Session: c.session, Data: data})
}

func (c *Client) SSVGetFairPlayContext() (*SSVGetContextReply, error) {
	return c.cli.SSVGetFairPlayContext(c.ctx, &SSVGetContextRequest{Session: c.session})
}

func (c *Client) SSVSubscriptionBag(ctxID uint32, dsid int64, txnType int32, amdm []byte) (*SSVSubBagReply, error) {
	return c.cli.SSVSubscriptionBag(c.ctx, &SSVSubBagRequest{Session: c.session, CtxId: ctxID, Dsid: dsid, TransactionType: txnType, Amdm: amdm})
}

func (c *Client) SSVIsValidContext(ctxID uint32) (*SSVIsValidContextReply, error) {
	return c.cli.SSVIsValidContext(c.ctx, &SSVContextHandle{Session: c.session, CtxId: ctxID})
}

func (c *Client) SSVSubscriptionRequest(ctxID uint32, dsid int64, txnType uint32, cert []byte) (*SSVSubscriptionRequestReply, error) {
	return c.cli.SSVSubscriptionRequest(c.ctx, &SSVSubscriptionRequestRequest{Session: c.session, CtxId: ctxID, Dsid: dsid, TxnType: txnType, Cert: cert})
}

func (c *Client) SSVImportKeybag(ctxID uint32, data []byte) (*SSVImportReply, error) {
	return c.cli.SSVImportKeybag(c.ctx, &SSVImportKeybagRequest{Session: c.session, CtxId: ctxID, Data: data})
}

func (c *Client) SSVImportSubKeybag(ctxID uint32, data []byte) (*SSVImportReply, error) {
	return c.cli.SSVImportSubKeybag(c.ctx, &SSVImportKeybagRequest{Session: c.session, CtxId: ctxID, Data: data})
}

func (c *Client) SSVImportSubResponse(ctxID uint32, ckc, subBag []byte) (*SSVImportReply, error) {
	return c.cli.SSVImportSubResponse(c.ctx, &SSVImportSubResponseRequest{Session: c.session, CtxId: ctxID, Ckc: ckc, SubBag: subBag})
}

func (c *Client) SSVStopSubscriptionLease(ctxID uint32) error {
	_, err := c.cli.SSVStopSubscriptionLease(c.ctx, &SSVContextHandle{Session: c.session, CtxId: ctxID})
	return err
}

func (c *Client) SSVDestroyFairPlayContext(ctxID uint32) error {
	_, err := c.cli.SSVDestroyFairPlayContext(c.ctx, &SSVContextHandle{Session: c.session, CtxId: ctxID})
	return err
}
