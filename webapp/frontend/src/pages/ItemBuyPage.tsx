import React from 'react';
import {withBaseComponent} from "../hoc/withBaseComponent";
import {ItemData} from "../dataObjects/item";
import ItemBuyFormContainer from "../containers/ItemBuyFormContainer";

interface ItemBuyPageProps {
    item: ItemData,
}

const ItemBuyPage: React.FC/*<ItemBuyPageProps>*/ = (/*{ item }*/) => {
    const errors = {
        cardError: '不正なカードです',
        buyError: '買えないよ',
    };
    const item = {
        id: 1,
        name: 'いす',
        price: 10000,
        description: 'いすです',
        createdAt: '2日前',
        thumbnailUrl: 'https://i.gyazo.com/c61ab08bca188410e81dbdcf7684e07e.png',
    };

    return (
        <React.Fragment>
            <ItemBuyFormContainer />
        </React.Fragment>
    );
};

export default withBaseComponent(ItemBuyPage);