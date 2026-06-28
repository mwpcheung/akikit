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

func (c *Client) RequestOTPForDSID(dsid int64) (*OtpReply, error) {
	return c.cli.RequestOTPForDSID(c.ctx, &MachineRequest{Session: c.session, Dsid: dsid})
}

func (c *Client) IsProvisioned(dsid int64) (*StatusReply, error) {
	return c.cli.IsProvisioned(c.ctx, &MachineRequest{Session: c.session, Dsid: dsid})
}

func (c *Client) ProvisionErase(dsid int64) (*StatusReply, error) {
	return c.cli.ProvisionErase(c.ctx, &MachineRequest{Session: c.session, Dsid: dsid})
}

func (c *Client) Synchronize(dsid int64, sim []byte) (*SynchronizeReply, error) {
	return c.cli.Synchronize(c.ctx, &SynchronizeRequest{Session: c.session, Dsid: dsid, Sim: sim})
}

func (c *Client) SetRoutingInfo(dsid int64, rinfo uint64) ([]byte, error) {
	r, err := c.cli.SetRoutingInfo(c.ctx, &SetRoutingInfoRequest{Session: c.session, Dsid: dsid, Rinfo: rinfo})
	if err != nil {
		return nil, err
	}
	return r.GetAdi(), nil
}

func (c *Client) GetRoutingInfo(dsid int64) (uint64, error) {
	r, err := c.cli.GetRoutingInfo(c.ctx, &GetRoutingInfoRequest{Session: c.session, Dsid: dsid})
	if err != nil {
		return 0, err
	}
	return r.GetRinfo(), nil
}

func (c *Client) ProvisionStart(dsid int64, spim []byte) (*ProvisionStartReply, error) {
	return c.cli.ProvisionStart(c.ctx, &ProvisionStartRequest{Session: c.session, Dsid: dsid, Spim: spim})
}

func (c *Client) ProvisionEnd(provSession uint64, ptm, tk []byte) ([]byte, error) {
	r, err := c.cli.ProvisionEnd(c.ctx, &ProvisionEndRequest{Session: c.session, ProvSession: provSession, Ptm: ptm, Tk: tk})
	if err != nil {
		return nil, err
	}
	return r.GetAdi(), nil
}

func (c *Client) ProvisionDestroy(provSession uint64) (*StatusReply, error) {
	return c.cli.ProvisionDestroy(c.ctx, &ProvisionDestroyRequest{Session: c.session, ProvSession: provSession})
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

func (c *Client) SAPTeardown(ctxh uint64) error {
	_, err := c.cli.SAPTeardown(c.ctx, &CtxHandle{Session: c.session, Ctx: ctxh})
	return err
}

func (c *Client) AbsinExchange1(cert []byte) (*AbsinExchange1Reply, error) {
	return c.cli.AbsinExchange1(c.ctx, &AbsinExchange1Request{Session: c.session, Cert: cert})
}

func (c *Client) AbsinExchange2(ctxh uint64, sessionData []byte) error {
	_, err := c.cli.AbsinExchange2(c.ctx, &AbsinExchange2Request{Session: c.session, Ctx: ctxh, SessionData: sessionData})
	return err
}

func (c *Client) AbsinSign(ctxh uint64, toSign []byte) (*AbsinSignReply, error) {
	return c.cli.AbsinSign(c.ctx, &AbsinSignRequest{Session: c.session, Ctx: ctxh, ToSign: toSign})
}

func (c *Client) AbsinTeardown(ctxh uint64) error {
	_, err := c.cli.AbsinTeardown(c.ctx, &CtxHandle{Session: c.session, Ctx: ctxh})
	return err
}

func (c *Client) KeyBagData(dsid int64, cval uint64) (*KeyBagDataReply, error) {
	return c.cli.KeyBagData(c.ctx, &KeyBagDataRequest{Session: c.session, Dsid: dsid, Cval: cval})
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
	return c.cli.SSVSubscriptionRequest(c.ctx, &SSVSubscriptionRequestRequest{Session: c.session, CtxId: ctxID, Dsid: dsid, TransactionType: txnType, Cert: cert})
}

func (c *Client) SSVImportKeybag(ctxID uint32, data []byte) (*SSVImportReply, error) {
	return c.cli.SSVImportKeybag(c.ctx, &SSVImportRequest{Session: c.session, CtxId: ctxID, Input: data})
}

func (c *Client) SSVImportSubscriptionKeybag(ctxID uint32, data []byte) (*SSVImportReply, error) {
	return c.cli.SSVImportSubscriptionKeybag(c.ctx, &SSVImportRequest{Session: c.session, CtxId: ctxID, Input: data})
}

func (c *Client) SSVImportSubscriptionResponse(ctxID uint32, ckc, subBag []byte) (*SSVImportReply, error) {
	return c.cli.SSVImportSubscriptionResponse(c.ctx, &SSVImportSubResponseRequest{Session: c.session, CtxId: ctxID, Ckc: ckc, SubBag: subBag})
}

func (c *Client) SSVStopSubscriptionLease(ctxID uint32) error {
	_, err := c.cli.SSVStopSubscriptionLease(c.ctx, &SSVContextHandle{Session: c.session, CtxId: ctxID})
	return err
}

func (c *Client) SSVFairplayDestroy(ctxID uint32) error {
	_, err := c.cli.SSVFairplayDestroy(c.ctx, &SSVContextHandle{Session: c.session, CtxId: ctxID})
	return err
}
