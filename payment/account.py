from algosdk import encoding, mnemonic
from nacl.signing import SigningKey
import base64

class Account:
    """Represents a private key and address for an Algorand account"""
    def __init__(self, *privateKey: str) -> None:
        if len(privateKey) == 1:
            self.signingKey = base64.b64decode(privateKey[0])[:32]
            self.verifyKey = base64.b64decode(privateKey[0])[32:]
            self.address = encoding.encode_address(self.verifyKey)
            self.privateKey = privateKey[0]
        else:     
            self.signingKey = SigningKey.generate()
            self.verifyKey = self.signingKey.verify_key
            self.address = encoding.encode_address(self.verifyKey.encode())
            self.privateKey = base64.b64encode(self.signingKey.encode() + self.verifyKey.encode()).decode()

    def getAddress(self) -> str:
        return self.address

    def getPrivateKey(self) -> str:
        return self.privateKey
    
    def getPublicKey(self) -> str:
        return self.verifyKey
    
    def getMnemonic(self) -> str:
        return mnemonic.from_private_key(self.privateKey)
    
    @classmethod
    def FromMnemonic(cls, m: str) -> "Account":
        return cls(mnemonic.to_private_key(m))
    